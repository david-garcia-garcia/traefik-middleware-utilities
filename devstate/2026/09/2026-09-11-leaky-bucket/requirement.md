# Requirement
IssueKey: 2026-09-11-leaky-bucket

## Problem
`master` has a Traefik token-bucket clock (`tokenbucket/`) and a Kong sliding-window hit counter (`windowcounter/`), but no classic leaky-bucket water clock. Callers that think in fill (errors, unhealthy backend) have nothing to import. README already tells them not to mix the two existing clocks and has no leaky row.

## Current (code)
- `leakybucket/` — not found.
- `bucket/` — not found.
- `ratelimit/` — not found.
- `tokenbucket/clock.go` — refill `tokens + rate×Δt`, cap `burst`, consume 1, refund when wait > `maxDelay`. Idle fills to burst. Inverse of water. Ticket forbids copying this refund.
- `tokenbucket/lua.go` — EVAL `HGETALL` then `HSET last,tokens` + `EXPIRE` (snapshot of `{last,tokens}` every Allow). `#rl_source == 4`. Not a leak-then-add-delta script.
- `tokenbucket/memory.go` — full in-process backend (`NewMemory`, mutex map). Not `x/time/rate`.
- `windowcounter/limiter.go` — sliding `estimated = current + previous × (1 − elapsed/window)`. `sync_rate=0` is `INCR` every Take; `>0` admits from `redisKnown + localDelta` and timer `EVAL` `INCRBY` + `EXPIREAT`-if-new. Local memory is only that buffer. `Sleep`/`Wake`/`Close` stop the flush goroutine. Min positive interval 20ms.
- `README.md` — libraries are reclaim, SimpleRedis, windowcounter, tokenbucket. Explicit: do not mix clocks. No leaky-bucket row.
- `simpleredis/simpleredis.go` — `Eval(script, keys, args) ([][]byte, error)` present. Errors `redis:unreachable` / `redis:timeout`.
- `reclaim/table.go` — `Hooks{Sleep, Wake, Close}` for Traefik reload.
- `.github/workflows/ci.yml` — `test` job starts Redis `:6379` and Dragonfly `:6380`; sets `WINDOWCOUNTER_LIVE_*` and `TOKENBUCKET_LIVE_*`. No leaky-bucket env.
- `tokenbucket/live_test.go` / `windowcounter/live_test.go` — table-driven live Redis/Dragonfly; skip under `-short` or unset env. Yaegi probes call `Allow`/`Take` (`useunsafe` false).
- `knowledge/research/ext_kong_rate-limiting_sliding-sync/` — Kong `sync_rate` + OSS `INCRBY` flush (delta + timer), not leaky water.
- `knowledge/research/ext_traefik_ratelimiter_token-bucket/` — Traefik Lua/in-memory token bucket (do not invert).
- `knowledge/research/ext_redis_eval/` / `ext_dragonfly_eval/` — EVAL `KEYS`; Dragonfly Lua 5.4 has no `table.maxn`.
- `knowledge/devdocs/std_go_windowcounter.md` — memory is only the `sync_rate` buffer; Peek/Take are window hits, not water.
- `knowledge/devdocs/std_go_tokenbucket.md` — Traefik Allow clock; not Kong `sync_rate`.
- `go.mod` — no `golang.org/x/time`.

## Desired
- New `leakybucket/` (`package leakybucket`). Import `github.com/david-garcia-garcia/traefik-middleware-utilities/leakybucket`.
- Clock: `water(t) = max(0, water(t0) + poured − leak×Δt)`, cap `capacity`. `Take`/`Add` pours `n` (default 1). Overflow → deny (do not pour). `Level` is current water after leak and does not pour. Empty bucket = room to burst up to `capacity`; then wait for leak. Not a sliding window.
- Opaque key; `Add`/`Take` → allowed + level (and optional time-until-not-full). No HTTP, no 429, no fail-open/fail-close, no health-gate. Caller prefixes keys.
- `sync_rate=0`: every pour is Redis `EVAL` (leak once on the server, then add). `>0` (seconds, min ~20ms): admit from `leaked(redis_water, last_sync, now) + local_pours`; timer `EVAL` applies leak once, adds `local_pours`, stores `{water, last}`. Never `SET` a replica’s `{water, last}` blob.
- Stores: in-memory (mutex, single process, exact) and Redis/Dragonfly. Same `leak` / `capacity`. Memory is a full backend; `local_pours` is only the `sync_rate>0` buffer.
- Redis via `simpleredis.Eval` only (script: leak, add delta, cap, `EXPIRE`). Keys in `KEYS`. No `table.maxn`. No `go-redis`. No `EVALSHA` in v1.
- Flush goroutine uses reclaim `Sleep`/`Wake`/`Close`.
- Redis errors: `redis:unreachable` / timeout. Policy stays in the middleware.
- README: add this library as the leaky-bucket row. Do not collapse into `tokenbucket/` or `windowcounter/`.
- Unit: fake TCP + in-memory leak (pour to cap → deny; idle → water drains; `Level` after Δt).
- Live Go e2e: Redis and Dragonfly; `go test` calls `Add`/`Take`/`Level` on those ports (no Traefik, no Pester). Prove `sync_rate=0` pour-to-cap then leak then Take; `sync_rate>0` two instances share one key with both pours counting (no last-write-wins) and over-allow bounded by the sync interval; in-memory and Redis agree on admit/deny for the same leak/capacity sequence when `sync_rate=0`; both engines (table-driven addr).
- Yaegi live: the same live scenarios interpreted (stdlib only, `useunsafe` false). Compiled test starts/skips engines; interpreted probe calls `Take`. CI starts both engines and must not skip.
- Pester/Traefik plugin optional, not a substitute.

## Affected
- `leakybucket/` (new)
- `README.md` (library table, layout, tests)
- `.github/workflows/ci.yml` (live env for this package; engines already exist)
- `knowledge/devdocs/` (usage packet in later phases)
- `openspec/specs/` (spec leaf in propose)

## Out of scope
- Traefik `maxDelay` refund, HTTP handler, 429, `time.Sleep` on delay.
- Kong window keys / sliding-window weights / `INCR` of window integers as the clock.
- `golang.org/x/time/rate`.
- `go-redis`, `EVALSHA` (later).
- Fail-open/fail-close, health-gate.
- Prefixing keys (caller’s job).
- Inverting `tokenbucket/` Lua (`tokens ≈ capacity − water` is the other way around).

## Unknowns
- Redis encoding of `{water, last}` (hash vs string vs two keys) while still leaking once then adding a delta in one EVAL.
- Whether time-until-not-full is a required return on `Add`/`Take` or optional.
- Whether `Add` and `Take` are aliases or `Take` is `n=1` only.
- Leak/capacity units (water per second; integer vs float) so in-memory and Redis agree.
- Live env var names and whether CI reuses the existing Redis/Dragonfly services.
- Whether the in-memory store also exposes `Sleep`/`Wake`/`Close` (no ticker) or only the Redis limiter does.

## Tensions
- Dest already replaced the old planned leaky-`bucket/` README row with `windowcounter/` and added `tokenbucket/` as a separate package. Add a leaky row; do not overwrite either sibling.
- `tokenbucket/` Redis `HSET`s a `{last,tokens}` snapshot every Allow. Doing the same for `{water, last}` from a replica is last-write-wins / double-leak — ticket forbids that SET. Steal Kong’s delta+timer (`windowcounter/` flush), not Traefik’s hash snapshot, and not Kong’s window math.
- `std_go_windowcounter.md` says local memory is only a buffer. This ticket locks memory as a full exact backend; the buffer is `local_pours` when `sync_rate>0`.
- Ticket still names `handoff-kong-window-limiter.md` / `handoff-traefik-token-limiter.md` as the test bar; those files may live only in a caller checkout. Dest already has the landed suites in `windowcounter/` and `tokenbucket/`.
