## Context

DestBranch (`origin/master`) already has `simpleredis.Eval`, `windowcounter/` (Kong sliding windows), and CI Redis `:6379` / Dragonfly `:6380` for that package. There is no `tokenbucket/`. README reserves a separate token-bucket package. Traefik facts: `knowledge/research/ext_traefik_ratelimiter_token-bucket/`. Explore decisions: `devstate/explore.md`. See proposal.md for why. Specs: `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`.

## Goals / Non-Goals

**Goals:**
- One Yaegi-safe `tokenbucket/` package with memory and Redis stores that share Traefik Lua math.
- Prove burst, refund, two-instance share, and memory/Redis agreement with `go test` (including Yaegi) on fake TCP and live engines.
- Wire CI so live tests do not skip.

**Non-Goals:**
- HTTP handler, 429, `time.Sleep` on delay, source extractor, `rate:` prefix.
- `go-redis`, `x/time/rate`, EVALSHA, Kong `sync_rate`, denyOnError.
- Reclaim hooks on the limiter in v1.
- Pester/Traefik plugin.
- Changing SimpleRedis or `windowcounter/`.

## Decisions

1. **Package `tokenbucket/`.** Dest siblings are folder=package. README forbids mixing clocks with `windowcounter/`. Alternative: `ratelimit/` or `bucket/` — rejected; those names hide which clock.

2. **Two constructors, one Allow.** `NewMemory(rate float64, burst int64, maxDelay, ttl time.Duration)` and `NewRedis(redis *simpleredis.SimpleRedis, rate, burst, maxDelay, ttl)`. Shared `Allow(key string) (bool, time.Duration, error)`. Alternative: one type that switches on nil redis — rejected; a memory limiter must not hold a Redis client.

3. **Lua formulas in memory.** Convert like Traefik Redis (`rate/1e6`, Unix microseconds, maxDelay microseconds). Do not import `x/time/rate`. Alternative: wrap `x/time/rate` — rejected; Yaegi and store agreement.

4. **Allow mapping in Go.** Script still returns `"true"`. Go sets allowed from wait vs maxDelay and from refund. Never sleep. Alternative: return wait only — rejected; Desired names allowed + wait.

5. **ttl required.** No Traefik `2s` / `1+1/rate` default. `ttl < 1s` fails New. Memory lazy-expires entries on Allow. Alternative: bake the HTTP formula — rejected; that formula is the caller's HTTP `New`.

6. **Copy Lua with `#rl_source == 4`.** Keep Traefik Labs MIT notice. `KEYS[1]` only. Parse Eval array wait as microseconds. Alternative: GET/SET hash from Go — rejected; race and Dragonfly undeclared keys.

7. **Test clock `SetNowForTest`.** Production uses `time.Now`. Name says test.

8. **Live addrs `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`.** Reuse CI services. Skip on short or empty. CI sets both, no `-short`. Alternative: reuse `WINDOWCOUNTER_LIVE_*` — rejected; packages skip independently.

9. **Yaegi GOPATH copies `tokenbucket` and `simpleredis`.** Compiled test starts/skips engines; interpreted probe calls Allow. stdlib only, `useunsafe` false.

## Risks / Trade-offs

- [Microsecond rounding vs wall clock] → Mitigation: injectable clock; agreement tests compare allowed and wait class, not exact nanoseconds.
- [Dragonfly `table.maxn` missing] → Mitigation: `#rl_source == 4`; live Dragonfly in CI.
- [Live tests skip in CI] → Mitigation: existing service containers + new env vars; no `-short` on the test job.
- [Memory map leak of unique keys] → Mitigation: lazy expire on Allow using caller ttl; no ticker.
- [Windows/local without Docker] → Mitigation: skip when addrs missing; CI still proves both engines.

## Migration Plan

New library. No production deploy. Rollback is revert the branch. Middleware authors import this after it lands; not this change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
