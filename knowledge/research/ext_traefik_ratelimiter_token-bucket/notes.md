# Traefik RateLimit token bucket

Facts about Traefik’s RateLimit middleware clock (token bucket). Not Kong window counters. Not this product’s wrappers. Pin: [traefik/traefik@903e8a9](https://github.com/traefik/traefik/tree/903e8a965795db5e750004ff74932983e269b85f).

## Clock (docs + HTTP wiring)

Docs: RateLimit is a token bucket. `average` / `period` define refill rate; `burst` is bucket size. Rate below 1 req/s uses `period` larger than a second. `average: 0` means no rate limiting. Redis is optional shared storage; without it the bucket is in-process. Source grouping (`sourceCriterion`) is HTTP-layer. There is **no** documented `delay` / `maxDelay` field — max wait is computed in Go. ([RateLimit docs](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/ratelimit/), [.sources/ratelimit-docs.md](.sources/ratelimit-docs.md))

`rate_limiter.go` computes `rtl = average * 1s / period` (reqs/s). `burst = max(config.Burst, 1)`. `period == 0` → 1s. `maxDelay` is `1/(2*rtl)` when `rtl >= 1`, else capped at 500ms. TTL is **2s** when `rtl >= 1`, else `1 + int(1/rtl)` when `0 < rtl < 1`. HTTP `Allow` then: error → 500; `delay == nil` → 429; `*delay > maxDelay` → 429 + `Retry-After` (no sleep); else `time.After(*delay)` then next. ([traefik@903e8a9:pkg/middlewares/ratelimiter/rate_limiter.go](https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/middlewares/ratelimiter/rate_limiter.go), [.sources/rate-limiter-go.md](.sources/rate-limiter-go.md))

**Conflict (docs vs Redis `average == 0`):** docs and in-memory `rate.Inf` allow all traffic. Redis `evaluateScript` short-circuits Inf to `(true, nil delay)`; `Allow` then returns `(nil, nil)`, which `ServeHTTP` maps to 429. Follow **source** for this pin; official docs describe the in-memory Inf case. ([traefik@903e8a9:redis_limiter.go](https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/middlewares/ratelimiter/redis_limiter.go), [.sources/redis-limiter-go.md](.sources/redis-limiter-go.md))

## Two stores, one meaning (why not GCRA)

In-memory: `golang.org/x/time/rate` `Limiter` in a TTL map (`maxSources = 65536`). `NewLimiter` starts **full** (tokens = burst). `Reserve()`, `Delay()`, `Cancel()` when delay > maxDelay; still returns `&delay` (HTTP 429s). `Reserve` not OK → `nil` delay (429 “No bursty traffic allowed”). ([traefik@903e8a9:in_memory_limiter.go](https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/middlewares/ratelimiter/in_memory_limiter.go), [.sources/in-memory-limiter-go.md](.sources/in-memory-limiter-go.md); [golang.org/x/time@v0.15.0:rate/rate.go](https://github.com/golang/time/blob/v0.15.0/rate/rate.go), [.sources/rate.go.md](.sources/rate.go.md))

Redis: EVAL of `AllowTokenBucketRaw` (same refill / burst / consume-1 / wait / refund-when-wait>maxDelay).

The pinned tree has **no** `GCRA` string (code search 2026-09-11). A product ticket that says Traefik rejected GCRA is not a tree-owned fact. Review on the first Redis PR rejected `go-redis/redis-rate` GCRA because in-memory is token bucket (`x/time/rate`); the same config struct must mean the same thing. ([traefik/traefik#10211](https://github.com/traefik/traefik/pull/10211), [.sources/pr-10211-gcra.md](.sources/pr-10211-gcra.md))

## Lua `AllowTokenBucketRaw`

Script ([traefik@903e8a9:lua.go](https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/middlewares/ratelimiter/lua.go), [.sources/lua-go.md](.sources/lua-go.md)):

- `KEYS[1]` = hash key. `HGETALL` / `HSET last,tokens` / `EXPIRE`.
- `ARGV`: `limit` (tokens per **microsecond** = reqs/s / 1e6), `burst`, `ttl` seconds, `t` Unix **microseconds**, `max_delay` microseconds.
- New/idle: empty hash → `tokens=0`,`last=0` → large elapsed → `min(delta, burst)-1` (idle fills to burst).
- Consume 1. If `tokens < 0`, `wait = -tokens / limit`. If `wait > max_delay`, refund 1 (cap burst).
- Returns `{tostring(true), wait_duration, tokens}` — first field is always `"true"`. Deny is HTTP comparing wait to maxDelay, not this boolean.
- Uses `table.maxn(rl_source) == 4` (Lua 5.1). Dragonfly is Lua 5.4 and does not polyfill `maxn` (`ext_dragonfly_eval`). Replacement: `#rl_source == 4` for this dense HGETALL array.

Go wrapper: `go-redis` v9.21.0 `script.Run` — optimistic **EVALSHA**, then **EVAL** on `NOSCRIPT`. Prefixes `rate:` + source. Passes `rate/1e6`, burst, ttl, `UnixMicro()`, `maxDelay.Microseconds()`. Redis errors return to HTTP as 500; no `denyOnError` in this package on this pin. Do not copy `Run`/EVALSHA. ([traefik@903e8a9:redis_limiter.go](https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/middlewares/ratelimiter/redis_limiter.go), [.sources/redis-limiter-go.md](.sources/redis-limiter-go.md); [go-redis@v9.21.0:script.go](https://github.com/redis/go-redis/blob/v9.21.0/script.go), [.sources/script.go.md](.sources/script.go.md))

Published `RateLimit` struct has Average, Period, Burst, SourceCriterion, Redis — **no Delay**. ([traefik@903e8a9:pkg/config/dynamic/middlewares.go](https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/config/dynamic/middlewares.go), [.sources/middlewares.go.md](.sources/middlewares.go.md))

## License

MIT. Copyright Containous SAS / Traefik Labs. Copied Lua must keep the notice. ([LICENSE.md](https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/LICENSE.md), [.sources/license-md.md](.sources/license-md.md))
