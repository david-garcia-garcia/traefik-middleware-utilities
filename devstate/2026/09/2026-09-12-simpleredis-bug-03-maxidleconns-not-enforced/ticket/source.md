# bug-03 — `MaxIdleConns` is silently not enforced

- **Axis**: Correctness (config not honoured)
- **Severity**: hard
- **Where**: `simpleredis/pool.go:139-145` (`release`), `simpleredis/config.go:28-29` (the documented contract)
- **Status**: not applied

## What I found

`Config.MaxIdleConns` is documented as "how many unused sockets release will keep".
It does not do that. The trim in `release` fires only when **both** halves of a
conjunction hold:

```go
sr.idleConnsMu.Lock()
// Close only when shut or idleConns is already maxIdleConns and live is at liveCap().
// inUse still includes this socket until freeInUseTurn runs.
idleConnsFull := len(sr.idleConns) >= sr.maxIdleConns
inUse := 0
if sr.inUseTurns != nil {
	inUse = sr.liveCap() - len(sr.inUseTurns)
}
live := len(sr.idleConns) + inUse
if sr.closed.Load() || (idleConnsFull && live >= sr.liveCap()) {
```

The second half is nearly unsatisfiable whenever `MaxIdleConns < PoolSize`, because
the comment on line 138 describes the reason: the socket being released still holds
its turn, so `inUse >= 1` and `live` is inflated by the very connection under
consideration. Measured by borrowing `PoolSize` connections and releasing them one
by one:

| PoolSize | MaxIdleConns | Final idle list | Honoured? |
|---|---|---|---|
| 8 | 2 | **7** | no |
| 16 | 1 | **15** | no |
| 2 | 8 | 2 | n/a (cap above pool) |
| 8 | 8 (default) | 8 | yes, trivially |

The per-release trace also shows the trim firing *arbitrarily* rather than
consistently — idle went `0→1→2→2→3→4→5→6→7` for `PoolSize 8 / MaxIdleConns 2`,
so one release out of eight closed its socket and the rest pooled it.

The reason it is erratic is the second defect here: `len(sr.inUseTurns)` is read
under `idleConnsMu`, but borrowers take tokens from the channel **without** that
lock. Channel length is not a data race, so `-race` would not flag it, but the
value is a stale sample of an unsynchronised counter. `inUse` is therefore a guess,
and the branch it feeds is timing-dependent.

The saving grace is that the *live* socket count is still correctly bounded by
`PoolSize` via the semaphore, so this is a config-honouring bug, not an unbounded
leak. Under concurrency the idle list did stay at or below `liveCap()`.

## Why it matters

`MaxIdleConns` is the only knob for "how many sockets may sit idle against Redis
per Traefik instance", and it silently does nothing in the configuration where
someone would reach for it. An operator who sets `PoolSize: 64, MaxIdleConns: 4`
to allow burst concurrency while keeping a small resting footprint gets **64**
resting sockets instead of 4.

Multiply by Traefik instances and by routers, and the resting connection count
against Redis is an order of magnitude above what was configured. Since there is
no reaper either ([risk-02](risk-02-no-idle-reaper.md)), those sockets stay
established indefinitely, each holding an 8 KiB `bufio` pair client-side and a
client output buffer server-side, and each counting against Redis `maxclients`.

The erratic trim is separately worth fixing because it makes the pool's behaviour
irreproducible. A test that asserts an idle-list size is inherently flaky today,
which is part of why this went unnoticed —
`../simpleredisfixes/test-02-idle-cap-and-release-after-close.md` records that the
cap path never executes in tests.

## Expected gain

Makes a documented knob work, and reduces resting sockets to what was asked for.
For the `PoolSize 64 / MaxIdleConns 4` shape that is a 16× reduction in idle
connections held against Redis.

Also makes pool behaviour deterministic, which is a prerequisite for asserting on
it in tests rather than logging and hoping.

## How to fix

The idle list is the thing `MaxIdleConns` is about, so trim on it alone and drop
the `live` term entirely:

```go
sr.idleConnsMu.Lock()
if sr.closed.Load() || len(sr.idleConns) >= sr.maxIdleConns {
	sr.idleConnsMu.Unlock()
	conn.close()
	sr.freeInUseTurn()
	return
}
sr.idleConns = append(sr.idleConns, conn)
sr.idleConnsMu.Unlock()
sr.freeInUseTurn()
```

This is safe precisely because the semaphore already caps live sockets: closing a
socket at the idle cap can never starve a borrower, since a borrower that cannot
find an idle socket dials one while holding its turn. The `live >= liveCap()`
condition was trying to prevent a starvation that the semaphore makes impossible.

Two follow-ons:

- **Delete `inUse`/`live` from `release`.** Once unused, the misleading read of
  `len(sr.inUseTurns)` outside its own synchronisation disappears with it. Keep
  `liveCap()` for `PoolSize()`, which is a legitimate use.
- **Reconsider the default.** `defaultMaxIdleConns` and `defaultPoolSize` are both
  8 (`config.go:6-7`), so the default configuration is unaffected by this fix. But
  once the cap works, `applyDefaults` should arguably clamp `MaxIdleConns` to at
  most `PoolSize` and document that a smaller value trades reconnects for a
  smaller footprint — which interacts with the churn measured in
  `../simpleredisfixes/perf-01-connection-pool-cap.md`.

## How to prove it

The deterministic test is the one that found this: borrow exactly `PoolSize`
connections directly, release them one at a time, and assert
`len(sr.idleConns) == min(MaxIdleConns, PoolSize)` at the end. Table it over
`8/2`, `16/1`, `2/8` and `8/8` so both the below-pool and above-pool cases are
pinned.

Because the current behaviour is timing-dependent, also assert it under
concurrency: many goroutines hammering commands, then quiesce and assert the idle
list settled at the cap. Sampling the peak during the run (as the audit harness
did) catches a fix that only converges after traffic stops.

Add a socket-count assertion against the fake server too — accepts minus closes —
so that a "fix" which merely hides connections from `idleConns` without closing
them cannot pass.
