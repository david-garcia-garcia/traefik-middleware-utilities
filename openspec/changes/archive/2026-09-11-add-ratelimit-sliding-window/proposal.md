## Why

Dest has SimpleRedis INCR/EXPIRE/EVAL and reclaim Sleep/Wake/Close, but no distributed window counter. README still plans a leaky `bucket/`. Middleware authors need a Kong-style sliding-window hit counter on Redis or Dragonfly, Yaegi-safe, proven with `go test` against both engines — not a token/leaky bucket and not a Traefik HTTP probe as the math suite.

## What Changes

- Add package `windowcounter/` (`package windowcounter`): caller injects `*simpleredis.SimpleRedis`; `New(redis, syncRate)`; `Take`/`Allow` on an opaque key + limit + window; sliding estimate only.
- Exact `sync_rate=0`: `Incr` every Take, `Expire` on first hit. Buffered `sync_rate>0`: admit from `redis_known + local_delta`; timer `Eval` INCRBY + EXPIREAT-if-new; 20 ms floor.
- Limiter `Sleep`/`Wake`/`Close` for `reclaim.Hooks`. Do not import `reclaim`. Do not `Close` the injected client.
- README library row and layout become this primitive (not leaky bucket).
- Unit tests with in-package fake TCP. Live Go e2e against Redis and Dragonfly (`Init` + `Take`). Yaegi live: same scenarios, compiled test owns start/skip.
- CI `test` job starts both engines and sets live addrs so the suite does not skip.
- Usage packet `knowledge/devdocs/std_go_windowcounter.md`.

## Capabilities

### New Capabilities

- `std_go_windowcounter_sliding-take`: Sliding-window `Take`/`Allow` — opaque key (caller owns identity), integer Redis counters for current and previous windows, estimate formula, error propagation, unit/Yaegi/live proof of admit/deny and boundary (no fixed-window double).
- `std_go_windowcounter_sync-flush`: `sync_rate` exact vs buffered flush — `Incr`+`Expire` vs EVAL INCRBY+EXPIREAT-if-new, two-client share without last-write-wins, Sleep/Wake/Close ticker lifecycle, CI starts Redis and Dragonfly.

### Modified Capabilities

- None. Dest `std_go_simpleredis_*` and `std_go_reclaim_*` requirements stay as they are. The limiter calls those APIs; it does not change them.

## Impact

- New `windowcounter/` (stdlib + this module’s `simpleredis` only).
- README Libraries/Layout/Tests. `.github/workflows/ci.yml` `test` job service containers for Redis and Dragonfly plus `WINDOWCOUNTER_LIVE_*` env.
- `knowledge/devdocs/std_go_windowcounter.md` and `index_std_go.md` row.
- No Pester/Traefik plugin. No `bucket/`. No `go-redis`. No EVALSHA. No fail-open/fail-close.
