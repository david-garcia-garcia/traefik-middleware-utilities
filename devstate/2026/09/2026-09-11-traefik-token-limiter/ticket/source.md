# Traefik token-bucket limiter

## Goal

A Yaegi-safe library that copies Traefik’s **token bucket** (same math in-process and on Redis/Dragonfly).

Sources (MIT — keep Traefik Labs attribution on copied Lua):

- Package: [`pkg/middlewares/ratelimiter`](https://github.com/traefik/traefik/tree/master/pkg/middlewares/ratelimiter)
- HTTP + in-memory wiring: [`rate_limiter.go`](https://github.com/traefik/traefik/blob/master/pkg/middlewares/ratelimiter/rate_limiter.go)
- Redis Lua: [`lua.go`](https://github.com/traefik/traefik/blob/master/pkg/middlewares/ratelimiter/lua.go) (`AllowTokenBucketRaw`)
- Redis Go wrapper: [`redis_limiter.go`](https://github.com/traefik/traefik/blob/master/pkg/middlewares/ratelimiter/redis_limiter.go)
- Docs: [RateLimit middleware](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/ratelimit/)

Do **not** copy Traefik’s HTTP handler, `go-redis`, `golang.org/x/time/rate` (Yaegi), or `time.Sleep` on delay. Do **not** implement Kong `sync_rate` / window counters here.

## Decisions (locked)

- **Clock:** Traefik token bucket: refill `rate` (reqs/s from `average/period`), cap `burst`, consume 1, compute `wait` if tokens &lt; 0. If `wait &gt; maxDelay`, refund (script) and deny. Idle source accumulates up to `burst`.
- **Two stores, one meaning:** in-memory map+mutex (stdlib reimplementation of the Lua / `x/time/rate` behaviour) **and** Redis `EVAL` of that script. Same `rate` / `burst` / `maxDelay` / `ttl`. This is why Traefik rejected GCRA.
- **Redis:** `simpleredis.Eval` only (script does `HGETALL` / `HSET` / `EXPIRE`). Keys in `KEYS`. **Replace `table.maxn`** (Lua 5.1; Dragonfly is 5.4) with `#rl_source == 4`. No `go-redis`, no `EVALSHA` in v1.
- **API:** opaque key; `Allow` → allowed + wait duration (caller 429s or waits). No HTTP, no source extractor, no `rate:` prefix (caller prefixes).
- **Redis errors:** return `redis:unreachable` / timeout. No denyOnError inside the library.
- **Package:** `tokenbucket/` (`package tokenbucket`). Import `github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket`. README: this is the leaky-bucket row’s replacement (token bucket, Traefik). Not `ratelimit/`, not `bucket/`.
- Blocked if `Eval` is missing. Do not GET/SET the hash from Go.

## Tests (required)

Same bar as the Kong handoff.

**Unit:** fake TCP for Eval encoding + in-memory bucket math (burst after idle, refund when wait &gt; maxDelay).

**Live Go e2e:** start **Redis and Dragonfly**, then `go test` calls `Allow` on those ports. No Traefik, no Pester.

Prove at least:

- burst: after idle, `burst` allows go through, next is delayed/denied per maxDelay
- two limiter instances share one Redis/Dragonfly key (no double burst)
- in-memory and Redis agree on admit/deny for the same rate/burst/maxDelay sequence
- **both** engines (table-driven addr)

**Yaegi live:** the **same live scenarios** interpreted (stdlib only, `useunsafe` false). Compiled test starts/skips engines; interpreted probe calls `Allow`. CI starts both engines and **must not** skip.

Pester/Traefik plugin is optional extra, not a substitute.
