# Token bucket

## Language

**Token bucket**:
A Traefik RateLimit clock. Package `tokenbucket`. Refill at `rate` tokens per second, cap at `burst`, consume 1 per Allow. It is not a sliding-window hit counter and not Kong `sync_rate`.
_Avoid_: naming the package `ratelimit` or `bucket`; mixing this clock with `windowcounter`; remaining quota

**Allow**:
One consume against an opaque key. Returns whether the consume is allowed and how long to wait. The caller 429s or waits. The library does not sleep.
_Avoid_: HTTP 429 inside the library; sourceCriterion; prefixing `rate:` inside the package

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket`. For Redis, inject an already-`Init`ed `*simpleredis.SimpleRedis`. Prefix keys in the caller. Compute `rate` from Traefik `average/period`. Pass `maxDelay` and `ttl` (Traefik uses `1/(2*rate)` capped at 500ms when rate < 1, and ttl `2s` when rate ≥ 1 else `1 + int(1/rate)`). Do not close the Redis client from the limiter.

## How to use

- `NewMemory(rate, burst, maxDelay, ttl)` or `NewRedis(client, rate, burst, maxDelay, ttl)` once.
- Call `Allow(ctx, key)` per request. Pass `req.Context()` on the request path; pass `context.Background()` when there is no deadline. Match Redis errors by `Error()` text.
- Prove with `go test -short ./tokenbucket/...`. Live Redis/Dragonfly is `limiter_e2e_test.go` / `limiter_yaegi_e2e_test.go` (see `knowledge/devdocs/std_go_test-suites.md`).

## Pattern snippet

```go
limiter, err := tokenbucket.NewRedis(client, 100, 50, 5*time.Millisecond, 2*time.Second)
if err != nil {
	return err
}
ctx := req.Context()
allowed, wait, err := limiter.Allow(ctx, "ip:" + ip)
if err != nil {
	return err
}
if !allowed {
	return errLimited
}
_ = wait
```

## Key files

- `tokenbucket/` — Traefik token bucket
- `openspec/specs/std_go_tokenbucket_allow/spec.md`, `openspec/specs/std_go_tokenbucket_lua-eval/spec.md`

## Gotchas

- `rate <= 0`, NaN, Inf, `burst < 1`, `maxDelay < 0`, or a `ttl` that is not a whole number of seconds of at least 1s fails New. Redis EXPIRE is integer seconds; New must not accept a Duration Redis cannot represent. Passthrough is skipping construction. This package has no unlimited-rate constructor.
- Lua always returns `"true"`; Go maps allowed from wait microseconds vs maxDelay microseconds (Lua refunds on `>`). The wait duration return is not the admit gate.
- A new or TTL-expired key starts at `burst`, then consume 1. Missing Redis hash does the same; do not assume `last=0` refill from Unix epoch.
- A 3-field Eval wait that is not a finite number (`nan`, `+Inf`, `-Inf`, `inf`) is `errEvalWait` (`tokenbucket: eval wait is not a number`), same as a garbage string. Do not treat it as admit or deny.
- EVAL scripts must list keys in `KEYS` (Dragonfly). Do not use `table.maxn`.
- Two limiter instances on one Redis key share burst. Do not expect a second full burst.
- Persisted `last` is never earlier than the previous `last`. Elapsed still clamps when now is behind `last` (no negative refill). Memory reads now after the mutex so lock order is clock order.
