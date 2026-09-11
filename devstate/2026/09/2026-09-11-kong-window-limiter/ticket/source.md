# Kong-style Redis window limiter

Pass this to explore, then implement. Think, then build. Do not ship token/leaky buckets.

## Goal

A Yaegi-safe library: **distributed HTTP rate limit** as a **windowed hit counter** ([Kong Rate Limiting Advanced](https://developer.konghq.com/plugins/rate-limiting-advanced); OSS Redis `sync_rate` + `INCRBY` flush: [`kong/plugins/rate-limiting/policies/init.lua`](https://github.com/Kong/kong/blob/master/kong/plugins/rate-limiting/policies/init.lua)). Redis- or Dragonfly-backed via `simpleredis`. Window math: [sliding vs fixed](https://developer.konghq.com/gateway/rate-limiting/window-types).

Not Traefik's token bucket. Not a leaky bucket. Not an in-memory limiter product. Local memory is only the **sync buffer**.

## Decisions (locked)

- **Clock:** sliding window. `estimated = current + previous × (1 − elapsed/window)`. Fixed window is out of v1.
- **Store:** Redis integers + TTL. Same RESP on Dragonfly (EVAL keys declared in `KEYS`; no `table.maxn`).
- **`sync_rate`:** Kong Advanced: `0` = exact (`INCR` every `Take`; expire on first hit). `>0` = seconds: admit from `redis_known + local_delta`, timer `EVAL` `INCRBY` + `EXPIREAT` if new. Min interval ~20ms if you need a floor.
- **API:** opaque key + limit + window; `Take`/`Allow` returns allowed + usage. Prefixing keys is the caller's job. No HTTP, no 429, no sleep.
- **Redis errors:** return `redis:unreachable` / timeout. No fail-open/fail-close inside the library. No health-gate.
- **Client:** `simpleredis` only (`Eval`, `Incr`/`IncrBy`, `Expire`/`ExpireAt`). No `go-redis`. If those methods are missing, this change is blocked — do not GET/SET-race a counter.
- **Timer:** reclaim `Sleep`/`Wake`/`Close` so the flush goroutine dies on Traefik reload. No leaked tickers.
- **Package:** `windowcounter/` (not `bucket/`, not `ratelimit/`). README row becomes this primitive.
- **EVALSHA:** later. `Eval` is enough.

## Tests (required)

**Unit:** fake TCP (existing SimpleRedis fake style) for encoder/window math without Docker.

**Live Go e2e (this is the behaviour suite):** a suite that **starts Redis and Dragonfly**, then **`go test` talks to them directly** — `Init` + `Take` against real ports. No Traefik, no Pester, no HTTP probe for limit math.

Prove at least:

- exact `sync_rate=0`: N `Take`s then deny; TTL so a new window admits again
- `sync_rate>0`: two clients (two `SimpleRedis` or two limiter instances) share one limit without last-write-wins
- sliding: dump-at-boundary does not double the limit the way a fixed window would
- same tests on **both** Redis and Dragonfly (table-driven backend addr)

**Yaegi live:** the **same live scenarios** run interpreted (`yaegi` stdlib only, `useunsafe` false, GOPATH copy of non-test sources). Compiled test owns process start / skip; interpreted probe calls `Take`. Skip the live files if the engines are not up (`testing.Short` or missing addrs) — CI must start both engines and **not** skip.

Pester/Traefik plugin is optional extra, not a substitute for this suite.
