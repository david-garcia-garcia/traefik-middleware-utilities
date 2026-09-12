# api-01 — Error sentinels are unexported, so callers must compare error text

- **Axis**: API design
- **Severity**: judgement
- **Where**: `simpleredis/simpleredis.go:11-28`, `windowcounter/limiter.go:271` (the consequence)
- **Status**: not applied

## What I found

The package exports the error *strings* but keeps every error *value* private:

```go
const (
	RedisUnreachable = "redis:unreachable"
	RedisMiss        = "redis:miss"
	// ...
)

var (
	errUnreachable = errors.New(RedisUnreachable)
	errPoolWait    = errors.New(RedisUnreachable)
	errMiss        = errors.New(RedisMiss)
	// ...
)
```

`errors.Is` is therefore impossible from outside the package, and the only
available check is string equality. The consumer does exactly that:

```go
// windowcounter/limiter.go:268-275
raw, err := l.redis.Get(redisKey)
if err != nil {
	if err.Error() == simpleredis.RedisMiss {
		return 0, nil
	}
	return 0, err
}
```

Note this is not caller sloppiness — it is the only option the API offers.

Two further consequences:

- **`==` on text is brittle.** The moment any `simpleredis` path wraps an error
  (`fmt.Errorf("%s: %w", …)`, which is the recommended fix in
  [bug-05](bug-05-windowcounter-hides-outage.md) for `parseEvalInt`), that
  comparison silently stops matching. Here the failure is severe: a `redis:miss`
  would stop being read as "counter absent, treat as zero" and start being treated
  as a hard error, so a limiter would deny traffic on every cache miss.
- **Callers cannot distinguish pool exhaustion from a dead server.**
  `errUnreachable` and `errPoolWait` are deliberately distinct values sharing one
  `Error()` text — a good design, since it keeps `shouldRetry` from multiplying
  `PoolTimeout` while letting callers match one token. But from outside, both are
  the string `redis:unreachable`, so a middleware cannot tell "Redis is down"
  (fail closed, alert) from "this instance is saturated" (shed load, raise
  `PoolSize`). Those warrant different responses.

## Why it matters

The consumers are rate limiters whose correctness depends on classifying Redis
errors precisely. `redis:miss` means "no counter yet, start at zero" — a normal,
extremely common case. Everything else means "I cannot enforce the limit". Getting
that boundary wrong flips a limiter between working and denying all traffic, and
the boundary is currently enforced by a string literal comparison that any future
error wrapping breaks silently at runtime with no compiler warning.

It also blocks improvements elsewhere in this backlog. Wrapping causes with `%w`
is the right fix for the opaque `redis:issue?` errors described in
[bug-05](bug-05-windowcounter-hides-outage.md), and for distinguishing reply-shape
failures in [risk-04](risk-04-non-basic-resp2-replies-rejected.md) — but wrapping
is precisely what breaks `err.Error() ==`. So the current API makes better error
reporting a breaking change instead of an additive one.

Rated judgement because nothing is broken today: the string comparison works, and
`errors.Is` on unwrapped sentinels is not yet needed. It is a design debt that
becomes a correctness bug the first time anyone wraps an error.

## Expected gain

Callers get `errors.Is`, which keeps working through arbitrary wrapping. That
unblocks adding context to errors without breaking consumers, and lets the two
`redis:unreachable` variants be told apart. No runtime cost — `errors.Is` on an
unwrapped sentinel is a pointer comparison.

## How to fix

Export the values and keep the strings for compatibility:

```go
// Exported sentinels. Match with errors.Is, not string equality.
var (
	ErrUnreachable = errors.New(RedisUnreachable)
	ErrMiss        = errors.New(RedisMiss)
	ErrTimeout     = errors.New(RedisTimeout)
	ErrNoAuth      = errors.New(RedisNoAuth)
	ErrIssue       = errors.New(RedisIssue)

	// ErrPoolWait is pool saturation. It wraps ErrUnreachable so callers matching
	// the broad condition keep working, while a caller that wants to shed load
	// rather than alert can match it specifically.
	ErrPoolWait = fmt.Errorf("%w", ErrUnreachable)
)
```

The internal names can simply become aliases, so no call site inside the package
changes. Verify one thing carefully when doing this: `shouldRetry` compares by
identity, and it depends on `errPoolWait != errUnreachable`. If `ErrPoolWait` is
built by wrapping, `errors.Is(ErrPoolWait, ErrUnreachable)` becomes **true**, so
any classifier rewritten in terms of `errors.Is` would start retrying pool waits —
exactly the behaviour the current design avoids. Either keep the classifiers on
identity, or check `ErrPoolWait` before `ErrUnreachable`. This is the one subtle
part of the change and deserves a test of its own.

Add predicates as the friendlier surface, which also keeps the Yaegi story simple
for interpreted consumers:

```go
func IsMiss(err error) bool        { return errors.Is(err, ErrMiss) }
func IsUnreachable(err error) bool { return errors.Is(err, ErrUnreachable) }
func IsPoolWait(err error) bool    { return errors.Is(err, ErrPoolWait) }
```

Then convert `windowcounter/limiter.go:271` to `simpleredis.IsMiss(err)` and keep
the string constants exported and documented as "for display and for legacy text
matching", so nothing that exists today breaks.

## How to prove it

The valuable test is the wrapping one, because it is the failure this prevents:
wrap each sentinel in `fmt.Errorf("context: %w", …)` and assert both `errors.Is`
and the corresponding predicate still match, while `err.Error() == Token` no longer
does. That documents in executable form why the change exists.

Pin the `ErrPoolWait`/`ErrUnreachable` relationship explicitly: assert
`errors.Is(ErrPoolWait, ErrUnreachable)` is true (broad matching works), that
`IsPoolWait` distinguishes them, and — critically — that `shouldRetry(ErrPoolWait)`
is still **false** while `shouldRetry(ErrUnreachable)` is **true**. That is the
regression this refactor most plausibly introduces.

On the consumer side, a `windowcounter` test where `Get` returns a *wrapped*
`redis:miss` and the limiter must still treat the counter as zero. That test fails
against today's `err.Error() ==` code, which is the point.

Finally, run the interpreted probes: consumers are Yaegi-interpreted, so add
`errors.Is` and one predicate to `clientprobe` in `simpleredis/yaegi_test.go`.
`errors.Is` is already used interpreted inside `ioError`, so this is expected to
pass — worth pinning, since the whole fix depends on it.
