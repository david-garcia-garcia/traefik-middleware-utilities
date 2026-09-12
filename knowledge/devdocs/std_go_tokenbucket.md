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
- Call `Allow(key)` per request. Match Redis errors by `Error()` text.
- Prove with `go test -short ./tokenbucket/...`. Live Redis/Dragonfly is `limiter_e2e_test.go` / `limiter_yaegi_e2e_test.go` (see `knowledge/devdocs/std_go_test-suites.md`).

## Pattern snippet

```go
limiter, err := tokenbucket.NewRedis(client, 100, 50, 5*time.Millisecond, 2*time.Second)
if err != nil {
	return err
}
allowed, wait, err := limiter.Allow("ip:" + ip)
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

- `rate <= 0`, `burst < 1`, `maxDelay < 0`, or `ttl < 1s` fails New. Passthrough is skipping construction.
- Lua always returns `"true"`; Go maps allowed from wait vs maxDelay and from refund.
- EVAL scripts must list keys in `KEYS` (Dragonfly). Do not use `table.maxn`.
- Two limiter instances on one Redis key share burst. Do not expect a second full burst.
