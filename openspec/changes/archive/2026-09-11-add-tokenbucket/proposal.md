## Why

Dest has SimpleRedis `Eval` and a Kong sliding-window hit counter, but no Traefik token-bucket primitive. Middleware authors who need refill + burst (same math in-process and on Redis/Dragonfly) have nowhere to import; mixing `windowcounter/` would be the wrong clock.

## What Changes

- Add package `tokenbucket/` (`package tokenbucket`): `NewMemory` and `NewRedis` with `rate` (reqs/s), `burst`, `maxDelay`, `ttl`; `Allow(key)` returns allowed + wait. Opaque key; caller prefixes. No HTTP, no source extractor, no sleep.
- In-memory store: stdlib map+mutex implementing Traefik Lua formulas (not `x/time/rate`). Redis store: `simpleredis.Eval` of the copied Traefik Lua (MIT attribution). Same `rate` / `burst` / `maxDelay` / `ttl`. Replace `table.maxn` with `#rl_source == 4`. No EVALSHA, no GET/SET of the hash from Go.
- Redis errors propagate (`redis:unreachable` / `redis:timeout`). No denyOnError. `New` rejects `rate <= 0`, `burst < 1`, `maxDelay < 0`, `ttl < 1s`.
- README: add a Token bucket row beside Window counter (do not overwrite the window-counter row).
- Unit tests with in-package fake TCP. Live Go e2e against Redis and Dragonfly (`TOKENBUCKET_LIVE_*`). Yaegi live: same scenarios, stdlib only, `useunsafe` false. CI sets both env vars on the existing engines.
- Usage packet `knowledge/devdocs/std_go_tokenbucket.md`.

## Capabilities

### New Capabilities

- `std_go_tokenbucket_allow`: Token-bucket `Allow` — opaque key (caller owns identity), Lua-formula refill/burst/wait/refund, memory store, admit/deny mapping, constructor validation, unit/Yaegi proof of burst-after-idle and refund.
- `std_go_tokenbucket_lua-eval`: Redis `EVAL` of the copied Traefik script — `KEYS`, `#rl_source == 4`, two limiter instances share one key, memory vs Redis agree, live Redis and Dragonfly, CI must not skip.

### Modified Capabilities

- None. Dest `std_go_simpleredis_*` and `std_go_windowcounter_*` requirements stay as they are. The limiter calls `Eval`; it does not change SimpleRedis. It does not change the window counter.

## Impact

- New `tokenbucket/` (stdlib + this module’s `simpleredis` only).
- README Libraries/Layout/Tests. `.github/workflows/ci.yml` `test` job adds `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY` (engines already exist).
- `knowledge/devdocs/std_go_tokenbucket.md` and `index_std_go.md` row.
- Copied Lua keeps Traefik Labs MIT notice.
- No Pester/Traefik plugin required. No `ratelimit/`. No `bucket/`. No `go-redis`. No EVALSHA. No fail-open/fail-close.
