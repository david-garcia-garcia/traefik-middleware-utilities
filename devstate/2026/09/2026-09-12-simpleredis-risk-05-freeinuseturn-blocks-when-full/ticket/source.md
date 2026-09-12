# risk-05 — `freeInUseTurn` blocks forever if a token is ever returned twice

- **Axis**: Liveness (latent)
- **Severity**: judgement
- **Where**: `simpleredis/pool.go:54-59` (`freeInUseTurn`)
- **Status**: not applied

## What I found

`freeInUseTurn` sends on the semaphore with no `default` and no timeout:

```go
func (sr *SimpleRedis) freeInUseTurn() {
	if sr.inUseTurns == nil {
		return
	}
	sr.inUseTurns <- struct{}{}
}
```

`inUseTurns` is buffered at `PoolSize` and pre-filled to full by
`ensureInUseTurns`. So a send that is not matched by a prior receive blocks
forever. Confirmed on a fresh, fully-stocked client:

```
fresh client tokens=2 cap=2
CONFIRMED: freeInUseTurn on a full semaphore blocks forever (goroutine leak / hang)
```

**No current code path does this.** I exercised token accounting across normal
cycles, dial failure, AUTH failure, pool-wait timeout, dirty sockets, and
`Close` racing with `release` — 24 goroutines × 4 client shapes, plus 200 forced
`release`/`Close` interleavings — and the semaphore was fully refilled every time.
The balance is correct today.

This is filed as the *failure mode* of a future accounting bug, not as a present
defect.

## Why it matters

It determines how a double-free manifests, and the current shape is the worst of
the three possibilities:

- **Blocking send (today):** the goroutine parks forever holding a Traefik request.
  No error, no log, no panic. Under sustained traffic, requests accumulate until
  the process runs out of memory. Nothing points at the pool.
- **Panic:** loud, immediate, and — after
  [bug-01](bug-01-panic-leaks-pool-token.md) is fixed — contained to one request.
- **Silent drop:** the pool loses a token, degrading capacity but staying alive.

The asymmetry matters because the pool's invariant is maintained by hand across
eight early returns in `borrow` and `release`, with no compiler help. Every future
change to those functions — bounded pipelining
(`../simpleredisfixes/perf-04-pipelining.md`), the `MaxIdleConns` fix in
[bug-03](bug-03-maxidleconns-not-enforced.md), or `context` cancellation in
[risk-01](risk-01-no-context-uncancellable-latency.md), which adds a whole new
early-exit path — is a chance to double-free. Cancellation is the most likely
culprit: a cancelled command that both returns early *and* is later released is
exactly this bug.

Rated judgement because nothing triggers it now. The reason to fix it anyway is
that the fix is two lines and converts an undebuggable hang into a visible fault.

## Expected gain

No behaviour change while the accounting is correct. When it is not, the symptom
becomes a detectable fault instead of a silent request pile-up — which is the
difference between finding the bug in an afternoon and never finding it.

## How to fix

Make the over-free non-blocking and detectable:

```go
func (sr *SimpleRedis) freeInUseTurn() {
	if sr.inUseTurns == nil {
		return
	}
	select {
	case sr.inUseTurns <- struct{}{}:
	default:
		// Over-free: the semaphore was already full, so some path returned a
		// turn it did not take. Dropping keeps the pool alive; the counter makes
		// the bug visible instead of hanging a request forever.
		sr.overFrees.Add(1)
	}
}
```

`atomic.Int64` for `overFrees`, matching the existing `atomic.Bool` field, which is
already proven to work as a struct field under Yaegi. Expose it as a method
(`OverFrees() int64`) alongside the existing `PoolSize()`/`MaxIdleConns()`
accessors so a test — or an operator — can read it.

The `default` branch is the safe choice over a panic: dropping a token cannot
corrupt anything, because the semaphore's job is to cap concurrency, and a full
semaphore already represents "nothing in use".

Belt-and-braces alternative, if a stronger guarantee is wanted: replace the
channel-as-semaphore with an explicit counter under `idleConnsMu`, which makes the
invariant checkable in one place instead of implied by channel arithmetic. That is
a larger change and would also fix the misleading `len(sr.inUseTurns)` read
described in [bug-03](bug-03-maxidleconns-not-enforced.md) — worth considering if
that fix is being done anyway.

## How to prove it

Two tests.

**The guard:** call `freeInUseTurn` on a fresh client and assert it returns
promptly and that `OverFrees()` is 1. Today the equivalent test has to prove the
*hang* by racing a goroutine against a timer and then draining the channel so the
test binary can exit — which is itself a sign the behaviour is wrong.

**The invariant:** the durable one. Hammer every borrow/release exit path from many
goroutines — healthy server, dead address, AUTH-rejecting server, and a starved
pool with a `PoolTimeout` short enough that the wait branch fires — then assert
`len(inUseTurns) == cap(inUseTurns)` **and** `OverFrees() == 0`. Both halves are
needed: the length check alone cannot distinguish "balanced" from "one leak plus
one over-free". Run it with `-race` once [ci-01](ci-01-no-race-detector-in-ci.md)
is fixed.

That invariant test is worth keeping permanently regardless of whether this
finding is fixed, since it is the only thing standing between a future refactor and
a silently degraded pool.
