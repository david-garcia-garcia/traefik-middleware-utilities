# perf-03 — Idle reaper only ever inspects the newest connection

- **Axis**: Performance
- **Severity**: judgement
- **Where**: `simpleredis/simpleredis.go:214-223` (`borrow` idle scan), `:250` (`release` timestamp)
- **Status**: not applied

## What I found

The idle list is a LIFO stack: `release` appends to the tail, and `borrow` pops
from the tail and stops at the first connection still inside `idleTimeout`:

```go
for len(sr.idle) > 0 {
	conn := sr.idle[len(sr.idle)-1]
	sr.idle = sr.idle[:len(sr.idle)-1]
	if now.Sub(conn.lastUsed) < idleTimeout {
		reused = conn
		break
	}
	stale = append(stale, conn)
}
```

Because the newest connection sits at the tail and the loop `break`s as soon as it
finds a fresh one, entries at the **head** of the list are never examined while
traffic keeps recycling the tail. A connection that went idle 10 minutes ago can
sit at index 0 indefinitely. There is no background reaper; the only other thing
that touches those entries is `Close`.

## Why it matters

Two costs, both bounded at 8 connections, which is why this is judgement and not
hard:

1. **Resources held for nothing.** Up to 8 sockets and 8 KB of `bufio` buffers each
   (a 4096-byte reader plus a 4096-byte writer, allocated in `dial` at `:271-272`),
   plus the corresponding client slot and output buffer on the Redis side, held
   open long past `idleTimeout`.
2. **A latency spike at the worst moment.** Redis closes idle clients per its own
   `timeout` setting, and load balancers and failovers do the same. Those
   head-of-list sockets are therefore likely to be *dead*, and the only time they
   get borrowed is when the idle list drains that far down — that is, during a
   concurrency spike. So the request that finally picks one up is a request
   arriving under load, and it pays a failed attempt plus a redial (see
   [test-01](test-01-server-closed-eof-redial.md) for the retry path). The stale
   connections are effectively landmines that only detonate during bursts.

The LIFO order itself is a reasonable choice — reusing the hottest connection is
what go-redis does too. The gap is that nothing ever ages out the cold end.

## Expected gain

- Removes up to 8 needlessly open sockets and ~64 KB of buffers per client
  instance during quiet periods, and the matching Redis-side client slots.
- Removes a class of burst-time latency spikes where a request pays
  attempt + redial because it drew a long-dead connection.
- No effect on the steady-state fast path: the tail connection is still reused
  with the same single comparison.

Modest and bounded — worth doing as part of the perf-01 pool work rather than on
its own.

## How to fix

Two options, either acceptable:

- **Sweep on release.** In `release`, opportunistically drop head entries older
  than `idleTimeout` before appending. Cheap, no goroutine, and it runs on every
  command so cold entries cannot accumulate. Keep it O(1)–O(2) per call by
  checking only the head, not the whole list.
- **Background reaper.** A ticker goroutine that trims expired idle connections,
  like go-redis's reaper. This costs a goroutine that must be stopped on `Close`,
  and the repo's handoff notes already call out that flush timers have to die on
  Traefik reload — so the sweep-on-release option carries less risk here.

Whichever is chosen, `Close` must still drain everything (it already does, `:64-78`).

## How to prove it

Backdate the `lastUsed` of a head entry while a fresh entry sits at the tail, run
a command, and assert the stale socket was closed rather than left in `sr.idle`.
`TestIdleTimeoutOpensANewConnection` covers only the single-entry case, where the
stale connection happens to also be the tail, which is why this gap survived.

## Index

From `simpleredisfixes/README.md`: review of `simpleredis/` for hot-path efficiency
and test coverage. Nothing in that folder is applied to `simpleredis/simpleredis.go`
yet. This ticket is finding perf-03 only.

## Caller addendum (conductor)

- Tests MUST run against both Redis and Dragonfly. Both are supported backends.
- Prove idle-head reap / no landmine on both live engines as well as the fake-server
  unit test in the finding.
- Extend compose + Pester `/redis` `/dragonfly` and `e2e/simpleredisprobe` if needed.
- Lua 5.1-safe. Dragonfly KEYS required.
- Prefer sweep-on-release unless dest evidence says otherwise.
