## Context

DestBranch (`origin/master`) already has `simpleredis.Eval`, `tokenbucket/` (Traefik refill snapshot HSET), `windowcounter/` (Kong sliding INCR/INCRBY + reclaim hooks), and CI Redis `:6379` / Dragonfly `:6380`. There is no `leakybucket/`. Meter facts: `knowledge/research/ext_leaky-bucket_meter/`. Kong flush facts: `ext_kong_rate-limiting_sliding-sync`. Explore decisions: `devstate/explore.md`. See proposal.md for why. Specs: `std_go_leakybucket_pour`, `std_go_leakybucket_sync-flush`.

## Goals / Non-Goals

**Goals:**
- One Yaegi-safe `leakybucket/` package with memory (exact) and Redis (exact EVAL or buffered delta) stores that share the I.371 water clock.
- Prove pour-to-cap, drain, two-instance no last-write-wins, bounded over-allow, and memory/Redis agreement with `go test` (including Yaegi) on fake TCP and live engines.
- Wire CI so live tests do not skip.

**Non-Goals:**
- HTTP handler, 429, `time.Sleep`, fail-open/fail-close, health-gate, key prefixing.
- `go-redis`, `x/time/rate`, EVALSHA, Traefik `maxDelay` refund, Kong window keys.
- Sleep/Wake/Close on the memory store.
- Pester/Traefik plugin.
- Changing SimpleRedis, `tokenbucket/`, or `windowcounter/`.

## Decisions

1. **Package `leakybucket/`.** Dest siblings are folder=package. README forbids mixing clocks. Alternative: fold into `tokenbucket/` or `bucket/` — rejected; water is the inverse of tokens and a different clock.

2. **Two constructors.** `NewMemory(leak float64, capacity float64, ttl time.Duration)` and `NewRedis(redis *simpleredis.SimpleRedis, leak, capacity, syncRate, ttl)`. Shared `Add`/`Take`/`Level`. Alternative: one type that switches on nil redis — rejected; a memory limiter must not hold a Redis client.

3. **Clock in microseconds.** `last` is Unix microseconds; leak is water per second; elapsed = Δμs / 1e6. Lua `tonumber` on the same ARGV. Alternative: whole-second leak — rejected; agreement tests would be coarse and tokenbucket already uses microseconds.

4. **HASH `{water, last}`.** One KEYS[1]. EVAL HGETALL, leak, maybe add, HSET, EXPIRE. Missing hash is water 0, last 0 (first pour: leak of empty is still empty). Alternative: string `water:last` or two keys — rejected; hash matches the sibling Redis shape without copying Traefik's every-Allow snapshot from Go. Alternative: GET then SET from Go — rejected; last-write-wins / double-leak.

5. **Add vs Take.** `Add(key, n int64)` pours n (`n < 1` errors). `Take(key)` calls Add 1. Alternative: both take n — rejected; Desired default 1 and Go has no default args.

6. **Until-not-full always returned.** Third value on Add/Take. Zero when `water + 1 <= capacity`. Alternative: omit — rejected; tokenbucket always returns wait and callers of a fill clock need a retry hint.

7. **ttl required.** Min 1s. Script EXPIRE that many seconds. Memory lazy-expires on later call. Alternative: derive ttl from `capacity/leak` — rejected; caller may want a longer key lifetime than drain time.

8. **Buffered admit.** Per key: `redisWater`, `lastSync`, `localPours`. Admit from leaked(redisWater, lastSync, now)+localPours. Timer EVAL ARGV = localPours + now + leak + capacity + ttl. Never HSET from Go. Over-allow is at most one interval of replica pours.

9. **Reclaim hooks only on Redis.** Copy `windowcounter` Sleep (flush+stop), Wake (restart ticker if sync_rate>0), Close (no redial of SimpleRedis). No `time.Tick`. Memory has no ticker.

10. **Test clock `SetNowForTest`.** Production uses `time.Now`. Name says test.

11. **Live addrs `LEAKYBUCKET_LIVE_REDIS` / `LEAKYBUCKET_LIVE_DRAGONFLY`.** Reuse CI services. Skip on short or empty. CI sets both, no `-short`. Alternative: reuse `TOKENBUCKET_LIVE_*` — rejected; packages skip independently.

12. **Yaegi GOPATH copies `leakybucket` and `simpleredis`.** Compiled test starts/skips engines; interpreted probe calls Take. stdlib only, `useunsafe` false.

## Risks / Trade-offs

- [Float Lua vs Go rounding] → Mitigation: agreement tests compare allow/deny and water class (empty / partial / full), not bit-identical floats.
- [Dragonfly `table.maxn` missing] → Mitigation: no `table.maxn`; `#` / explicit field counts; live Dragonfly in CI.
- [Buffered over-allow] → Mitigation: spec bounds it by the sync interval; exact mode for callers who cannot tolerate it.
- [Live tests skip in CI] → Mitigation: existing service containers + new env vars; no `-short` on the test job.
- [Memory map leak of unique keys] → Mitigation: lazy expire on call using caller ttl; no ticker.
- [Windows/local without Docker] → Mitigation: skip when addrs missing; CI still proves both engines.

## Migration Plan

New library. No production deploy. Rollback is revert the branch. Middleware authors import this after it lands; not this change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
