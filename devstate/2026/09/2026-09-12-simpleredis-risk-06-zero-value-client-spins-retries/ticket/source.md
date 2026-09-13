# risk-06 — A client built without `New` spins the retry ladder instead of failing fast

- **Axis**: API misuse resistance
- **Severity**: judgement
- **Where**: `simpleredis/pool.go:67-69` (`borrow`), `simpleredis/commands_exec.go:18-24` (`exec`)
- **Status**: not applied

## What I found

`SimpleRedis` is an exported struct with unexported fields, so `&SimpleRedis{}`
compiles. `New` is the only thing that calls `ensureInUseTurns`, so a zero-value
client has a nil semaphore. `borrow` handles that:

```go
if sr.inUseTurns == nil {
	return nil, errUnreachable
}
```

Returning `errUnreachable` is the safe choice — but it is the one error
`shouldRetry` treats as **retryable**, and `exec` only short-circuits when
`sr.isClosed()`, which a zero-value client is not. So the misconfiguration takes
the full retry ladder:

```
zero-value Get: err=redis:unreachable elapsed=53.0428ms (default retry ladder is 3 retries, 8ms..512ms)
CONFIRMED: nil-semaphore client spins the retry ladder (53.0428ms) instead of failing fast
```

Every exported method survives without panicking — `Get`, `Set`, `Del`, `MGet`,
`Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX`, `MSetEXAt`, and `Close`
twice. The accessors also degrade sensibly, because each falls back to its default:
`PoolSize()` reports 8, `IdleTimeout()` 30 s, and so on. So the struct is
defensively written; the gap is purely that a structurally-impossible client is
reported as a *transient network* condition.

Also note `ensureInUseTurns` is exported-adjacent in effect: calling it on a
zero-value client silently builds a `defaultPoolSize` pool against an empty
`host`, producing a client that dials `""` forever.

## Why it matters

53 ms per call, on every call, forever — with an error that says the network is at
fault. In a Traefik middleware that is 53 ms added to every request on the route,
and the operator's investigation starts at Redis and their firewall rather than at
their own construction code.

The realistic route to this is not someone typing `&SimpleRedis{}` on purpose. It is:

- A struct literal in a consumer that stores `SimpleRedis` by value or builds it
  field-by-field, then loses the `New` call in a refactor.
- A test double or fixture that constructs the struct directly.
- A future `Config`-reload path that rebuilds the client in place.

Both current consumers hold `*simpleredis.SimpleRedis` and receive it injected
(`tokenbucket/redis.go:14`, `windowcounter/limiter.go:26`), so neither can hit this
today. It is a guardrail, not an active bug — hence judgement.

The deeper point is that "no semaphore" and "Redis is down" are different kinds of
fact. One is a programming error that will never fix itself; the other is transient
and worth retrying. Collapsing them into one retryable sentinel is what turns a
clear failure into a slow, misattributed one.

## Expected gain

A misconstructed client fails on the first call, immediately, with an error that
names the cause. Removes 53 ms × every request of pointless latency in the failure
case, and removes a misleading diagnosis.

## How to fix

Distinguish "structurally unusable" from "unreachable", the same way `errPoolWait`
is already kept distinct from `errUnreachable` while sharing its text:

```go
// errNotInitialised is a client that did not come from New. Error() is
// redis:unreachable so callers still match that token, but identity keeps it out
// of shouldRetry: no amount of retrying will create a semaphore.
var errNotInitialised = errors.New(RedisUnreachable)
```

Return it from `borrow`'s nil check. Because `shouldRetry` compares by identity
(`err == errUnreachable`), this is automatically non-retryable — the existing
mechanism needs no change, which is a good sign the design was right. Consumers
matching on the `redis:unreachable` string keep working.

Optionally make the misuse harder in the first place:

- **Fail loudly instead.** Have `exec` treat a nil semaphore as a programming error
  and panic with a clear message. Defensible for a "you must call `New`" contract,
  but a panic in a Traefik plugin is worse than a fast error, and after
  [bug-01](bug-01-panic-leaks-pool-token.md) it is contained but still noisy.
  Not recommended.
- **Make the zero value unusable by construction.** An unexported required field,
  or returning an interface from `New`, prevents the mistake at compile time. Both
  are larger API changes and the interface option conflicts with the deliberate
  choice to expose a concrete type for Yaegi's sake.
- **Validate `Host`.** `New` accepts an empty `Host` today and produces a client
  that dials `""`. Rejecting it — or documenting that `New` cannot fail and
  deferring the error to first use, which is the current contract — closes the
  adjacent hole.

The `errNotInitialised` change alone is two lines and gets most of the value.

## How to prove it

Assert the *latency*, not just the error. A zero-value client's `Get` must return
in well under one backoff interval, and the error must be
`simpleredis.RedisUnreachable`. Checking only the error text passes today.

Add a smoke test that calls every exported method on `&SimpleRedis{}` and asserts
none panics, including `Close` twice. That test exists in spirit in the audit and
is worth keeping: it is the cheapest guard against a future field being
dereferenced without a nil check, which is a real risk as `borrow`/`release` gain
paths.

Assert `cap(sr.inUseTurns) == 0` for the zero value, to pin that `New` remains the
only constructor of the semaphore — that is the invariant the fix depends on.
