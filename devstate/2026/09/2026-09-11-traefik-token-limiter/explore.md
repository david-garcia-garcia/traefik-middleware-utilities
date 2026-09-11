# Explore
IssueKey: 2026-09-11-traefik-token-limiter

## Concepts

- **Token bucket (Traefik clock)** — refill `rate` tokens/s, cap `burst`, consume 1 per `Allow`. If tokens go negative, `wait = -tokens / rate`. If `wait > maxDelay`, refund 1 and deny. Idle source fills to `burst`. Not Kong sliding windows (`windowcounter/`). Not GCRA.
- **Two stores, one meaning** — in-memory map+mutex (stdlib, not `golang.org/x/time/rate`) and Redis `EVAL` of the copied Lua. Same `rate` / `burst` / `maxDelay` / `ttl`. Traefik rejected GCRA on the Redis path because in-memory is token bucket (`knowledge/research/ext_traefik_ratelimiter_token-bucket/`, PR 10211).
- **Lua `AllowTokenBucketRaw`** — `KEYS[1]` hash; `HGETALL` / `HSET last,tokens` / `EXPIRE`. ARGV: `limit` = reqs/s / 1e6, `burst`, `ttl` seconds, Unix **microseconds**, `max_delay` microseconds. First return field is always `"true"`; deny is wait vs maxDelay in Go. Replace `table.maxn` with `#rl_source == 4` (Dragonfly Lua 5.4).
- **Allow** — opaque key in, `(allowed bool, wait time.Duration, err error)` out. Library does not HTTP 429, sleep, or extract source. Caller prefixes keys (`rate:` is not ours). Redis errors stay `redis:unreachable` / `redis:timeout`.
- **SimpleRedis.Eval** — dest already has it (`simpleredis/simpleredis.go` `Eval`). Blocked-if-missing is closed. No GET/SET of the hash from Go. No EVALSHA.
- **Sibling pattern** — `windowcounter/` live env + Yaegi GOPATH interp + fake TCP. Copy that test bar, not the clock.

Gap measured (2026-09-11, worktree `2026-09-11-traefik-token-limiter`): `tokenbucket/` not found. README line 25 reserves a separate token-bucket package and tells callers not to mix clocks. `Eval` is present. CI already starts Redis `:6379` and Dragonfly `:6380` for windowcounter.

```
  Allow(key)
       │
       ├─ memory: mutex map, Lua formulas, lazy TTL expire
       └─ redis:  simpleredis.Eval(copied Lua, KEYS=[key])
              │
              ▼
     allowed + wait (caller 429s or waits)
```

## Decisions

- New package `tokenbucket/` (`package tokenbucket`). Import `github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket`. Do not fold into `windowcounter/` or name it `ratelimit/` / `bucket/`.
- Public constructors: `NewMemory(rate, burst, maxDelay, ttl)` and `NewRedis(redis *simpleredis.SimpleRedis, rate, burst, maxDelay, ttl)`. Shared `Allow(key string) (bool, time.Duration, error)`. `rate` is reqs/s (`float64`); caller computes `average/period`. `burst` is `int64`. `ttl` is integer seconds at Redis (`EXPIRE`); memory uses the same duration for lazy expire.
- In-memory implements the **Lua formulas** (microsecond `limit`, Unix micro now, refund when wait > maxDelay). Do not import `x/time/rate`. Injectable clock (`SetNowForTest`) so memory and Redis sequences can agree in tests.
- Copy Traefik Lua with Traefik Labs MIT notice. Keys in `KEYS`. `#rl_source == 4`. Parse Eval array: wait is microseconds → `time.Duration`.
- README: **add** a Token bucket row beside Window counter. Do not overwrite the window-counter row (dest already replaced leaky-`bucket/` with `windowcounter/`).
- Tests: fake TCP for Eval encoding; in-memory burst-after-idle and refund; live `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY` on the existing CI engines; Yaegi GOPATH, stdlib only, `useunsafe` false; CI sets both env vars and must not skip. Pester plugin optional, not a substitute.
- Spec family (propose): new `std_go_tokenbucket_*` leaves. Usage packet later: `std_go_tokenbucket.md`.
- No HTTP handler, source extractor, `time.Sleep` on delay, `go-redis`, EVALSHA, Kong `sync_rate`, denyOnError, or GET/SET of the token hash from Go.

## Open questions

- Q: Who already owns the client address / source identity used as the Allow key?
  Rank: additive asked — Desired: opaque key; no source extractor
  Decision: resolved — none in this library. The middleware (Traefik `sourceCriterion`) owns IP / header / user. Caller builds and prefixes the key. Do not read HTTP or reconstruct the address.
  By: explore

- Q: How does `Allow`'s `allowed` bool map Traefik Lua (always `"true"`) plus HTTP 429 when wait > maxDelay or delay is nil?
  Rank: additive asked — Desired: Allow → allowed + wait duration; refund and deny if wait > maxDelay
  Decision: assumed — Lua still returns `"true"`; Go maps: `allowed=false, wait=0` when the store cannot reserve (no burst); `allowed=false, wait>maxDelay` after refund (caller may set Retry-After); `allowed=true, wait>=0` when admitted now or after waiting ≤ maxDelay. Library never sleeps.
  By: explore

- Q: Microsecond Lua clock vs nanosecond `x/time/rate` — which unit so both stores agree?
  Rank: additive asked — Desired: two stores, one meaning
  Decision: assumed — public API is `time.Duration` + reqs/s. Both stores convert like Traefik Redis (`rate/1e6`, `UnixMicro`, `maxDelay.Microseconds()`). In-memory uses those formulas, not `x/time/rate`.
  By: explore

- Q: Live env var names, and does CI reuse the existing Redis/Dragonfly pair?
  Rank: additive asked — Desired live Go e2e + Yaegi live; both engines; CI must not skip
  Decision: assumed — `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`. Reuse CI services `127.0.0.1:6379` and `127.0.0.1:6380`. Skip without addrs or under `-short`. CI sets both.
  By: explore

- Q: Do in-memory buckets need reclaim `Sleep`/`Wake`/`Close`?
  Rank: additive incidental — ticket does not require a ticker; Traefik uses a TTL map
  Decision: assumed — no reclaim hooks on the limiter in v1. Memory map expires entries on `Allow` using the caller `ttl` (lazy). Callers who store a limiter in reclaim pass their own table hooks around the instance. No per-key ticker.
  By: explore

- Q: Default ttl if the caller does not pass Traefik’s formula?
  Rank: additive asked — Desired locks ttl as a library parameter
  Decision: assumed — no default. `ttl < 1s` fails `New`. Document Traefik’s formula in usage (`2s` when rate ≥ 1, else `1 + int(1/rate)`). Do not bake it.
  By: explore

- Q: What happens when `rate <= 0` (Traefik `average: 0` / `rate.Inf`)?
  Rank: additive incidental — not in Desired; Traefik Redis Inf currently 429s on this pin
  Decision: assumed — `New` returns an error when `rate <= 0` or `burst < 1` or `maxDelay < 0`. Do not copy Inf short-circuit. Passthrough is the caller skipping construction.
  By: explore
