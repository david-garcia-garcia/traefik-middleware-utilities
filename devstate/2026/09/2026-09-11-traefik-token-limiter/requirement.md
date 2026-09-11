# Requirement
IssueKey: 2026-09-11-traefik-token-limiter

## Problem
`master` has SimpleRedis `Eval` and a Kong sliding-window hit counter, but no Traefik token-bucket primitive. Middlewares that need refill + burst (same math in-process and on Redis/Dragonfly) have nowhere to import. README already points that clock at a future separate package.

## Current (code)
- `tokenbucket/` — not found.
- `bucket/` — not found.
- `ratelimit/` — not found.
- `windowcounter/limiter.go` — Kong sliding `Take`/`Allow` on Redis integers + optional `sync_rate` buffer. Not a token bucket. README tells callers not to mix clocks.
- `README.md` — libraries are reclaim, SimpleRedis, windowcounter. Explicit: not Traefik RateLimit, not token bucket; that primitive would be a separate package. No leaky-`bucket/` row left.
- `simpleredis/simpleredis.go` — `Eval(script, keys, args) ([][]byte, error)` present. Errors `redis:unreachable` / `redis:timeout`.
- `windowcounter/fake_redis_test.go` — in-process TCP RESP fake for unit tests (EVAL path is the Kong flush script, not token-bucket HGETALL/HSET).
- `windowcounter/live_test.go` — live `go test` against Redis and Dragonfly via `WINDOWCOUNTER_LIVE_REDIS` / `WINDOWCOUNTER_LIVE_DRAGONFLY`.
- `windowcounter/yaegi_test.go` — interpreted `Take` (stdlib, `useunsafe` false); live Yaegi uses the same env vars; `-short` skips live.
- `.github/workflows/ci.yml` — `test` job starts Redis `:6379` and Dragonfly `:6380` and sets the windowcounter env vars; Pester job still optional for Traefik plugins.
- `knowledge/devdocs/std_go_windowcounter.md` — avoid naming a package `tokenbucket`; this ticket creates that package on purpose.
- `knowledge/research/ext_redis_eval/`, `ext_dragonfly_eval/` — EVAL KEYS + no `table.maxn`.
- `knowledge/research/ext_traefik_ratelimiter_token-bucket/` — Traefik Lua/in-memory clock (written this prepare). Not a product package.

## Desired
- New `tokenbucket/` (`package tokenbucket`). Import `github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket`.
- Opaque key; `Allow` → allowed + wait duration. Caller 429s or waits. No HTTP, no source extractor, no `rate:` prefix.
- Clock: Traefik token bucket — refill `rate` (reqs/s), cap `burst`, consume 1, wait if tokens < 0, refund and deny if wait > maxDelay. Idle fills to burst.
- Two stores, one meaning: in-memory map+mutex (stdlib, not `x/time/rate`) and Redis `simpleredis.Eval` of the copied Lua (same rate/burst/maxDelay/ttl).
- Lua: keep Traefik Labs MIT attribution. Keys in `KEYS`. Replace `table.maxn` with `#rl_source == 4`. No `go-redis`, no `EVALSHA` in v1. Do not GET/SET the hash from Go. Blocked if `Eval` missing (it is not).
- Redis errors: return `redis:unreachable` / timeout. No denyOnError in the library.
- README: add this library as the Traefik token-bucket row (not `ratelimit/`, not `bucket/`). Do not collapse it into `windowcounter/`.
- Unit: fake TCP for Eval encoding + in-memory math (burst after idle; refund when wait > maxDelay).
- Live Go e2e: start Redis and Dragonfly; `go test` calls `Allow` on those ports (no Traefik, no Pester). Prove burst-after-idle, two instances sharing one key (no double burst), in-memory vs Redis admit/deny agreement, both engines (table-driven addr).
- Yaegi live: same scenarios interpreted (stdlib only, `useunsafe` false). Compiled test starts/skips engines; interpreted probe calls `Allow`. CI starts both engines and must not skip.
- Pester/Traefik plugin optional, not a substitute.

## Affected
- `tokenbucket/` (new)
- `README.md` (library table, layout, tests)
- `.github/workflows/ci.yml` (live env for this package; engines already exist for windowcounter)
- `knowledge/devdocs/` (usage packet in later phases)
- `openspec/specs/` (spec leaf in propose)

## Out of scope
- Traefik HTTP handler, source extractor, `time.Sleep` on delay, 429 responses inside the library.
- `go-redis`, `golang.org/x/time/rate`, `EVALSHA`.
- Kong `sync_rate` / window counters (`windowcounter/` already owns that).
- denyOnError / fail-open / fail-close.
- Prefixing keys (`rate:` is the caller’s job).
- GET/SET of the token hash from Go.

## Unknowns
- How `Allow`’s `allowed` bool maps Traefik’s Lua (always returns `"true"`) plus HTTP 429 when wait > maxDelay / nil delay.
- Microsecond Lua clock (`UnixMicro`, rate/1e6) vs nanosecond `x/time/rate` — which unit the stdlib in-memory port uses so both stores agree.
- Live env var names and whether CI reuses the existing Redis/Dragonfly services or adds a second pair.
- Whether in-memory buckets need reclaim `Sleep`/`Wake`/`Close` (ticket does not require a ticker; Traefik uses a TTL map).
- Default ttl if the caller does not pass Traefik’s `1s + slack` formula.

## Tensions
- Ticket still talks as if README’s leaky-`bucket/` row is the one to replace. Dest already replaced that row with `windowcounter/` and reserved token bucket as a **separate** package — add a row, do not overwrite the window-counter row.
- `std_go_windowcounter.md` says avoid the name `tokenbucket`; this ticket locks that package name for the other clock.
- Traefik computes `maxDelay` and `ttl` in HTTP `New`; the ticket locks them as library parameters (caller can copy that formula).
- Traefik in-memory uses `x/time/rate` (Yaegi-unsafe); ticket forbids copying it and asks for a stdlib map+mutex that matches the Lua.
