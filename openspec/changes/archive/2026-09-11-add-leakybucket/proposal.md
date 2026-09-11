## Why

Dest has a Traefik token-bucket clock and a Kong sliding-window hit counter, but no classic leaky-bucket meter. Middleware authors who think in fill (errors, unhealthy backend) have nothing to import; inverting `tokenbucket/` Lua or `SET`ting a replica `{water, last}` blob would be the wrong clock and a last-write-wins leak.

## What Changes

- Add package `leakybucket/` (`package leakybucket`): `NewMemory(leak, capacity, ttl)` and `NewRedis(client, leak, capacity, syncRate, ttl)`. `Add(key, n)` pours `n`; `Take(key)` is Add 1; `Level(key)` does not pour. Returns allowed, water level, and time-until-not-full. Opaque key; caller prefixes. No HTTP, 429, fail-open/fail-close, health-gate, or sleep.
- Clock: `water(t) = max(0, water(t0) + poured − leak×Δt)`; overflow denies and does not pour. Empty bucket is room up to `capacity`. In-memory mutex map is a full exact backend. Redis: `simpleredis.Eval` leak-once-then-add on a HASH `{water, last}`. `sync_rate=0` EVAL every pour; `>0` admits from leaked Redis water plus `local_pours` and a timer flush. Go never `SET`/`HSET` that hash from a replica.
- Redis errors propagate (`redis:unreachable` / `redis:timeout`). Reclaim `Sleep`/`Wake`/`Close` on the Redis limiter so the flush goroutine dies on Traefik reload. No `EVALSHA`, no `go-redis`, no `table.maxn`.
- README: add a Leaky bucket row beside Token bucket and Window counter (do not overwrite either).
- Unit tests with in-package fake TCP. Live Go e2e against Redis and Dragonfly (`LEAKYBUCKET_LIVE_*`). Yaegi live: same scenarios, stdlib only, `useunsafe` false. CI sets both env vars on the existing engines.
- Usage packet `knowledge/devdocs/std_go_leakybucket.md`.

## Capabilities

### New Capabilities

- `std_go_leakybucket_pour`: Leaky-bucket pour — opaque key (caller owns identity), water clock, overflow deny without pouring, `Add`/`Take`/`Level`, memory store, until-not-full, constructor validation, unit/Yaegi proof of pour-to-cap then leak.
- `std_go_leakybucket_sync-flush`: Redis EVAL leak-then-add (HASH `water`/`last`, KEYS, no replica SET) plus Kong-style `sync_rate` delta timer; two instances share one key; memory vs Redis agree when `sync_rate=0`; live Redis and Dragonfly; CI must not skip.

### Modified Capabilities

- None. Dest `std_go_tokenbucket_*` and `std_go_windowcounter_*` stay as they are. This package calls `Eval`; it does not change SimpleRedis. It does not invert Traefik Lua or reuse window keys.

## Impact

- New `leakybucket/` (stdlib + this module’s `simpleredis` and `reclaim.Hooks` shape only).
- README Libraries/Layout/Tests. `.github/workflows/ci.yml` `test` job adds `LEAKYBUCKET_LIVE_REDIS` / `LEAKYBUCKET_LIVE_DRAGONFLY` (engines already exist).
- `knowledge/devdocs/std_go_leakybucket.md` and `index_std_go.md` row.
- No Pester/Traefik plugin required. No `ratelimit/`. No `bucket/`. No merge into `tokenbucket/` or `windowcounter/`. No `x/time/rate`. No EVALSHA.
