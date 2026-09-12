# bug-01 — A panic between `borrow` and `release` deadlocks the pool for the life of the process

- **Axis**: Correctness / liveness
- **Severity**: critical
- **Where**: `simpleredis/commands_exec.go:26-27` (`exec`), `simpleredis/pool.go:54-59` (`freeInUseTurn`)
- **Status**: not applied

## What I found

`exec` borrows a pool token, runs the command, and hands the token back by calling
`release`. That call is **not deferred**:

```go
conn, err := sr.borrow()
if err != nil {
	if sr.isClosed() || !shouldRetry(err) {
		return nil, err
	}
	last = err
	continue
}
values, reusable, err := sr.do(conn, args)
sr.release(conn, reusable)   // <- the only path that returns the token
```

`release` is the sole caller of `freeInUseTurn`, which is the sole way a token
re-enters `inUseTurns`. So any panic raised inside `do` — anywhere in
`SetDeadline`, `writeCommand`, or the whole RESP parser — unwinds straight past
line 27. The token is gone permanently, and so is the socket.

Measured against a server that returns a reply header the parser panics on
(`PoolSize: 2`):

```
before:              tokens=2 cap=2
call 1 panicked:     runtime error: makeslice: len out of range
after call 1:        tokens=1 cap=2
call 2 panicked:     runtime error: makeslice: len out of range
after call 2:        tokens=0 cap=2
borrow after leak:   err=redis:unreachable after 200.2121ms (PoolTimeout=200ms)
```

After `PoolSize` panics the semaphore is empty and **every subsequent command
fails**, each after burning the full `PoolTimeout` first. The client never
recovers; there is no reaper, no token refill, and no way to reset short of
rebuilding the client.

[bug-02](bug-02-unbounded-reply-allocation.md) is a live, server-triggerable source
of exactly this panic, which is why these two are ranked together.

## Why it matters

This is the worst-shaped failure in the library, because of how Traefik handles
panics. A plugin panic is recovered per request, so the process **survives** and
keeps serving traffic. What dies silently is the Redis client: a handful of
malformed replies converts it into an object that returns `redis:unreachable`
forever while Redis itself is perfectly healthy.

The observable symptoms all point away from the cause:

- Redis dashboards are green; the client is not even connecting.
- Every request now pays `PoolTimeout` before failing, so latency rises by a
  fixed 200 ms per request rather than failing fast.
- Restarting Redis changes nothing. Only restarting Traefik clears it.
- With `PoolSize` at its default of 8, **eight** malformed replies are enough.

For the rate limiters built on this client, a bricked pool means every `Take`
errors. Depending on the consumer's policy that is either a full outage
(fail-closed) or an unlimited bypass (fail-open) — see
[bug-05](bug-05-windowcounter-hides-outage.md).

The token accounting is otherwise correct: I exercised normal cycles, dial
failure, AUTH failure, pool-wait timeout and dirty sockets, and every one of
those paths conserved the semaphore. This is specifically the unwind path, and it
is the one path a `defer` would have covered for free.

## Expected gain

Converts an unrecoverable, silent, permanent client failure into a single failed
request. No fast-path cost: `defer` on an already-allocated call is negligible
next to a network round trip, and the interpreted overhead is one deferred call
per command against ~35 µs of Yaegi interpretation per command.

It also removes an entire class of future bug. Any panic added to the parser or
the write path later — including one introduced by the pipelining work in
`../simpleredisfixes/perf-04-pipelining.md` — is contained automatically instead
of bricking the pool.

## How to fix

Make token return unconditional:

```go
conn, err := sr.borrow()
// ...
var (
	values   [][]byte
	reusable bool
)
func() {
	defer func() { sr.release(conn, reusable) }()
	values, reusable, err = sr.do(conn, args)
}()
```

Two details matter. `reusable` must be read by the deferred closure *after* `do`
returns, so a normal completion still pools a clean socket. And on a panic
`reusable` is left at its zero value `false`, which is exactly right: a socket
abandoned mid-command has unknown framing and must be closed, not pooled.

Whether to also swallow the panic is a separate decision. Recommended: do **not**
recover in `exec`. Let it propagate so the bug stays visible in Traefik's logs,
and fix the panic sources properly ([bug-02](bug-02-unbounded-reply-allocation.md)).
Recovering here would trade a loud bug for a quiet one. If a recover is added
later, convert it to `redis:issue?` and do not retry.

While here, consider making `freeInUseTurn` non-blocking as defence in depth —
see [risk-05](risk-05-freeinuseturn-blocks-when-full.md).

## How to prove it

A static fake that replies with a header the parser panics on (`*1000000000000000000\r\n`
suffices today), a client with a small `PoolSize`, and a loop of `PoolSize`
recovered calls. Assert `len(sr.inUseTurns) == cap(sr.inUseTurns)` afterwards.

That assertion is the durable one: it holds regardless of which panic source
exists, so the test keeps protecting the invariant after bug-02 is fixed. Pair it
with a follow-up `borrow()` asserting it still succeeds, which is what actually
demonstrates the pool is alive. The existing token-conservation harness described
in `../simpleredisfixes/test-02-idle-cap-and-release-after-close.md` is the right
place to add it.
