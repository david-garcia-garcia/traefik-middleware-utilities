# perf-01 — No cap on total connections; bursty traffic redials constantly

Bound: this finding only (`simpleredisfixes/perf-01-connection-pool-cap.md`). Sibling findings in `simpleredisfixes/` are other tickets; do not implement them.

Index row: Performance, severity hard — no cap on total connections; bursty traffic redials constantly.

## Caller addendum (must record in Desired/Affected; do not implement in prepare)

Test coverage for this change MUST run against both Redis and Dragonfly. Fake-server tests are not a substitute. Follow dest's compose + Pester e2e (`Test-Integration.ps1`, `/redis` and `/dragonfly`, `e2e/simpleredisprobe`) and extend it so this ticket's new behavior is proven on both engines. Lua 5.1-safe (no `table.maxn`). Dragonfly requires keys listed in KEYS. CI must exercise both backends.

## Finding

- **Axis**: Performance
- **Severity**: hard
- **Where**: `simpleredis/simpleredis.go:26` (`maxIdleConns`), `:204-242` (`borrow`), `:245-260` (`release`)
- **Status**: not applied

### What I found

`maxIdleConns = 8` caps only the **idle list**, not the number of live sockets.
`borrow` pops an idle connection if one is there and otherwise dials
unconditionally.

`release` then refuses to pool anything above the cap and closes it.

So the live socket count equals the number of concurrent callers, with no upper
bound, and every socket beyond the 8th is destroyed the moment its command
finishes. There is no wait queue: a caller never waits for a free connection, it
always opens its own.

Measured with a fake server given 500 µs of latency so callers genuinely overlap
(`TestConnectionChurnAcrossBursts`, `TestConnectionChurnUnderLatency`):

| Traffic shape | Commands | Dials | Ideal |
|---|---|---|---|
| 5 bursts of 64 concurrent `Get` | 320 | **276** | 64 |
| 64 callers looping steadily | 1,280 | 66 | 64 |

Steady concurrency is fine — a released connection is grabbed by the next caller
before the idle list fills. Bursty concurrency is not: 86% of commands paid a full
TCP handshake.

### Why it matters

Real HTTP traffic is bursty by nature, which is the shape that behaves worst here.
Three costs stack up on the affected requests:

1. **A TCP handshake per request.** One extra round trip before the command.
2. **`AUTH` and `SELECT` per request** when `Init` was given a password or database
   (`dial` at `:263-289`). Those are two *more* sequential round trips, each a
   full `do` call with its own deadline. A passworded Redis over a 0.5 ms network
   turns a ~0.5 ms `Get` into a ~2 ms `Get` for 86% of requests.
3. **Unbounded fan-out.** 500 concurrent Traefik requests means 500 sockets to
   Redis, each holding an 8 KB `bufio` reader/writer pair client-side plus a
   client output buffer server-side. Nothing in the library refuses to grow, so
   Redis `maxclients` is the only backstop — and hitting it fails *all* callers,
   not just the excess.

This is the finding that turns "Redis got slow" into "Traefik hammered Redis with
thousands of new connections", which is why it is ranked first.

### Expected gain

- **Eliminates ~86% of dials** under bursty load (measured 276 → ~64 for the same
  320 commands). Each avoided dial saves a TCP handshake, and two extra round
  trips whenever a password or database is configured.
- **Bounds Redis-side resources** to a known number instead of scaling with
  Traefik's concurrency.
- No change in the steady-state fast path: a warm pooled connection is still a
  single `SetDeadline` + write + read.

### How to fix

Follow go-redis's pool shape (`internal/pool/pool.go`): a total connection cap
with a wait queue and a pool timeout, rather than an idle-only cap.

- Add a `poolSize` (total live connections) separate from the idle cap, defaulting
  to something like 8–16, and a buffered channel used as a semaphore so callers
  above the cap wait for a free connection instead of dialing.
- Add a `poolTimeout`: if no connection frees up in that window, return
  `redis:unreachable` (or a distinct `redis:pool-timeout`) instead of queueing
  without bound. Failing fast locally beats overwhelming Redis.
- Keep the idle cap for trimming, but stop closing a connection just because the
  idle list is momentarily full while total live connections are under `poolSize`.

Keep this Yaegi-safe: a buffered `chan struct{}` and `select` with a
`time.After`/timer are plain stdlib and work interpreted.

### How to prove it

`TestConnectionChurnAcrossBursts` already measures the thing that must improve;
turn its `t.Logf` into an assertion once the cap exists (dials should stay at or
near `poolSize` across bursts). Add a test that concurrency above the cap never
exceeds `poolSize` live sockets, and one where all connections are busy and the
pool timeout elapses, asserting the error rather than an unbounded dial.
