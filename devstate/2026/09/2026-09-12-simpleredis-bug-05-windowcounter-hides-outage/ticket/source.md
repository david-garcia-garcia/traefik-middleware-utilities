# bug-05 — Buffered `windowcounter` hides a Redis outage and multiplies the global limit

- **Axis**: Correctness (consumer) / security
- **Severity**: hard
- **Where**: `windowcounter/limiter.go:350` (`flushLoop`), `:285` (`Sleep`), `:308` (`Close`), `:235-241` (`windowLocked`)
- **Status**: not applied

## What I found

In buffered mode (`syncRate > 0`) the flush error is discarded in all three places
it can surface:

```go
// flushLoop, limiter.go:344-355
for {
	select {
	case <-ticker.C:
		_ = l.flushPending()      // :350
	case <-stop:
		return
	}
}
```

`Sleep` (`:285`) and `Close` (`:308`) do the same. `flushPending` itself is
careful — it collects `firstErr` and only resets `state.localDelta = 0` on success
— but nothing ever reads what it returns.

On its own that would merely be unreported. It becomes a bypass because of how
`windowLocked` decides whether to consult Redis:

```go
if state.localDelta == 0 {
	known, err := l.getCount(redisKey)   // only path that can observe the outage
	if err != nil {
		return nil, err
	}
	state.redisKnown = known
}
```

With an unflushed delta pending, `Take` consults neither window key, so it never
learns Redis is gone. The trigger is therefore **timing-dependent**, and my first
attempt to reproduce it failed for that reason:

| State when Redis dies | Next `Take` | Behaviour |
|---|---|---|
| `localDelta == 0` (just flushed) | GETs, fails | fails **closed**, error reported — honest |
| `localDelta > 0` (between flushes) | touches nothing | returns **nil error**, admits locally |

Measured for the second row, `limit=5`, Redis killed after one hit with the delta
still local:

```
outage Take #2 : allowed=true  estimate=2.0 err=<nil>
outage Take #5 : allowed=true  estimate=5.0 err=<nil>
outage Take #6 : allowed=false estimate=6.0 err=<nil>
...
limit was 5; during the outage 11/11 Takes returned a NIL error and 4 were admitted
flushPending() actually returns: redis:unreachable
```

So the accurate characterisation is **a silent degradation window between flush
ticks**, not a blanket fail-open. Per-instance enforcement continues (`#6` onward
denied), because `localDelta` keeps accumulating when flushes fail. What is lost
is coordination and any error signal.

Exact mode (`syncRate == 0`) is honest by contrast: `takeExact` propagates the
`Incr`/`Expire` error and fails closed. Verified.

## Why it matters

Each instance enforces `limit` against its own local counter, so with *N* Traefik
instances the effective global limit becomes roughly **`limit × N`** for as long
as the outage lasts, with **no error surfaced to any caller** — not to the
middleware, not to a log, not to a metric. `Take` returns `nil`, so from above the
limiter it is indistinguishable from healthy operation.

For a rate limiter that is the security-relevant direction of failure. Two things
sharpen it:

- **The window is always open.** `minSyncRate` floors `syncRate` at 20 ms, so the
  gap between "delta flushed" and "delta pending" is short — but a hot key has a
  pending delta almost continuously, which is exactly the key an attacker is
  hammering. Low-traffic keys sit at `localDelta == 0` and fail closed; the busiest
  key is the one that degrades.
- **The mode silently changes the failure contract.** `syncRate == 0` fails closed
  and reports; `syncRate > 0` fails open-ish and stays silent. That is a large
  semantic difference selected by what reads as a performance knob, and it is not
  documented in the README's description of the limiter.

This also interacts with [bug-01](bug-01-panic-leaks-pool-token.md): a bricked
pool looks exactly like an outage, so the same silent degradation applies with a
perfectly healthy Redis.

## Expected gain

Restores an error signal on the one code path where a rate limiter's guarantee can
silently lapse, and lets the middleware above choose its own policy (fail open,
fail closed, or shed) with knowledge instead of by accident. No hot-path cost:
storing a `error` on flush failure is one assignment per tick, not per request.

## How to fix

Retain the flush error and let callers see it. Keep the decision explicit rather
than silently changing `Take`'s behaviour:

```go
// on the Limiter, guarded by l.mu
lastFlushErr error
flushFailedAt time.Time
```

Set both in `flushLoop`, `Sleep` and `Close` instead of discarding, then pick one
of:

- **Surface it from `Take`** as a third return value or a typed error that still
  carries the admit decision, so the middleware can decide. This is the most
  honest, and the most invasive.
- **Expose `LastFlushError()` / `Stale() bool`** and let the middleware poll it.
  Least invasive, and enough for a middleware to fail closed or emit a metric.
- **Add a staleness deadline**: if no flush has succeeded for *k* × `syncRate`,
  make `Take` start returning the error itself. This bounds the divergence window
  without requiring the caller to do anything, and is the best default.

Whichever is chosen, **document the per-mode failure semantics** next to
`syncRate` in `New` and in the README, because the current asymmetry between exact
and buffered mode is the part most likely to surprise.

Two adjacent cleanups worth folding in:

- `windowLocked` skipping the `GET` whenever `localDelta > 0` is the mechanism
  that hides the outage. A staleness deadline is a better fix than forcing a
  `GET`, which would undo the whole point of buffering.
- `parseEvalInt` at `:397` and `:401` throws away the underlying
  `strconv.ParseInt` failure and the reply that failed to parse, so a limiter that
  starts denying traffic reports only the bare string `redis:issue?`. Wrap the
  cause: `fmt.Errorf("%s: %w", simpleredis.RedisIssue, convErr)`. See also
  [api-01](api-01-sentinels-not-exported.md), which is what makes wrapping awkward
  today.

## How to prove it

A killable in-process RESP server is the key harness — one that answers normally,
then closes its listener and every live socket on demand. The test must control
flush timing, because the bug only exists in one of the two states:

1. `syncRate` long enough that no tick fires. One `Take` while healthy (leaving
   `localDelta == 1`), kill Redis, then `Take` repeatedly. Assert the error is
   **not** nil — this is the test that fails today, 11 times out of 11.
2. The mirror case: `Take`, let a flush succeed, kill Redis, `Take`. Assert it
   fails closed with an error. This one passes today and must keep passing, since
   it is the honest path.
3. Exact mode with the same outage, asserting the error propagates. Guards against
   a fix that accidentally makes exact mode lenient.

Add a two-instance test for the coordination claim: two `Limiter`s on two clients
sharing one Redis, killed mid-window, asserting the *combined* admitted count
against `limit`. That is the assertion that actually encodes the product
requirement, and it is the one that would have caught this.
