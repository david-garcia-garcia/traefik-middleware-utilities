# risk-02 — Nothing reaps idle sockets; `IdleTimeout` only applies to the newest one

- **Axis**: Resource management
- **Severity**: judgement
- **Where**: `simpleredis/pool.go:109-125` (`takeIdleConn`), `simpleredis/simpleredis.go:61-79` (`New`)
- **Status**: not applied
- **Overlaps**: `../simpleredisfixes/perf-03-idle-reaper-tail-only.md` — same root cause, measured here

## What I found

`New` starts no goroutines. Verified by counting: `runtime.NumGoroutine()` was
identical before and after `New`. `IdleTimeout` is therefore not a reaper — it is a
gate applied lazily, only when a later `borrow` happens to inspect a socket.

Measured with `IdleTimeout: 40ms`, six sockets pooled, then traffic stopped:

```
idleConns: right after traffic=6, 500ms later (IdleTimeout=40ms)=6
server sockets: accepted=6, open after traffic=6, open 500ms later=6
```

Twelve times past the idle timeout, all six sockets are still established.

Compounding it, `takeIdleConn` pops from the **tail** and returns on the first
young socket it finds:

```go
for len(sr.idleConns) > 0 {
	conn := sr.idleConns[len(sr.idleConns)-1]
	sr.idleConns = sr.idleConns[:len(sr.idleConns)-1]
	if now.Sub(conn.lastUsed) < sr.idleTimeout {
		return conn, stale, false
	}
	stale = append(stale, conn)
}
```

Since `release` appends, the tail is the most recently used socket, so the scan
almost always succeeds on its first iteration. Sockets at the *head* — the oldest,
the ones `IdleTimeout` is actually about — are never examined while traffic
continues. LIFO reuse is the right choice for latency; the problem is that nothing
else ever visits the head.

`Close` does close the whole list, so this is not a leak across a client's
lifetime. It is a leak across *idle time*, and — combined with nothing calling
`Close` (see [build-01](build-01-test-binary-does-not-build.md) context and the
audit note below) — across a client's abandonment.

## Why it matters

Up to `PoolSize` sockets stay established per client indefinitely, holding an
8 KiB `bufio` reader/writer pair each client-side, a client output buffer
server-side, and a slot against Redis `maxclients`. With
[bug-03](bug-03-maxidleconns-not-enforced.md) that is `PoolSize` rather than the
configured `MaxIdleConns`, so the resting footprint is larger than intended too.

The practical failure it invites is stale-socket reuse. Redis's own `timeout`
setting closes idle client connections server-side; since this client never
proactively reaps, the first command after a quiet period reuses a socket the
server already dropped. That path is handled correctly today — I verified a
server-closed pooled socket is transparently redialled via the retry — but it
converts a would-be free command into a failed one plus a redial, and it does so
in bursts after every quiet period.

Severity is judgement rather than hard because the count is bounded by `PoolSize`
and the failure mode degrades gracefully. It matters most in the Traefik shape:
many routers, each with a client, most idle most of the time.

## Expected gain

Resting connections drop to zero after `IdleTimeout` instead of staying at
`PoolSize`. Eliminates the post-quiet-period redial burst, and removes the
interaction with Redis-side `timeout` entirely.

## How to fix

Two options, and the second is preferable for this library.

**A background reaper.** A single goroutine per client on a ticker, closing sockets
past `IdleTimeout`. Correct and simple, but it means `New` starts a goroutine that
only `Close` stops — and since nothing in the repo calls `Close` outside tests
today, that trades a socket leak for a goroutine leak. Only viable together with
disciplined lifecycle ownership (`reclaim` is the repo's mechanism for this).

**Full-list sweep on borrow, keeping LIFO reuse.** No goroutine, no lifecycle
requirement:

```go
// takeIdleConn: sweep the whole list, then reuse the newest survivor
now := time.Now()
survivors := sr.idleConns[:0]
for _, conn := range sr.idleConns {
	if now.Sub(conn.lastUsed) < sr.idleTimeout {
		survivors = append(survivors, conn)
		continue
	}
	stale = append(stale, conn)
}
sr.idleConns = survivors
if n := len(sr.idleConns); n > 0 {
	reused = sr.idleConns[n-1]
	sr.idleConns = sr.idleConns[:n-1]
}
return reused, stale, false
```

This is O(idle) under `idleConnsMu` instead of O(1), but `idle` is bounded by
`MaxIdleConns` (once [bug-03](bug-03-maxidleconns-not-enforced.md) is fixed), and
the existing code already closes `stale` sockets outside the lock — keep that.

It does not help a client that goes completely idle, since nothing calls `borrow`.
If that case matters, combine the sweep with a reaper whose lifetime is owned by
`reclaim`, which is what the repo built `reclaim.Hooks{Close: ...}` for.

## How to prove it

The measurement above is the test: pool *n* sockets, stop traffic, wait past
`IdleTimeout`, and assert the fake server observes all *n* closed. Assert on the
server's accept-minus-close count rather than on `len(idleConns)`, so a fix that
merely forgets connections without closing them fails.

For the tail-only defect specifically, the assertion needs sockets of *differing*
ages: pool several, age only the head ones (by manipulating `lastUsed`), then
`borrow` once and assert the aged head sockets were closed — not just that the
borrow returned a young socket. A test that only checks the returned connection
passes today.

Also assert `runtime.NumGoroutine()` is unchanged across `New` if the sweep option
is chosen, or that it returns to baseline after `Close` if the reaper option is.
