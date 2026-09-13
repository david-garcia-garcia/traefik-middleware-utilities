## Why

Dest `Allow` can persist a `last` in the past: `consumeOne` clamps elapsed when now is behind `last` but still stores now, and Lua `HSET` writes `t` the same way. Memory also samples the clock before `mu.Lock()`, so a stale now can overwrite a newer `last`. The next consume refills an interval already granted.

## What Changes

- Tests first: `TestRepro_StaleNowRewindsLastDoubleRefill` (`sequential` and `goroutines_stale_samples_before_fresh_lock`) on dest `Allow(ctx, key)`, plus a script assertion that Lua HSET last is persist-max, not raw `t`. Confirm FAIL, then fix, then PASS.
- `consumeOne` persists `max(previous last, nowMicro)` and keeps the elapsed clamp.
- Lua `HSET last` is `max(bucket.last, t)`. Keep the Traefik Labs MIT notice. This field is no longer bit-identical to Traefik.
- Memory reads `now` after `mu.Lock()` so lock order is clock order.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: persisted `last` SHALL NOT rewind; elapsed still clamps so it is never negative; Memory samples now after the mutex.
- `std_go_tokenbucket_lua-eval`: Eval `HSET last` is `max(bucket.last, t)` while elapsed still uses the backward clamp. MIT notice kept.

## Impact

- `tokenbucket/clock.go` (`consumeOne` persisted last)
- `tokenbucket/lua.go` (`HSET last`; MIT notice stays)
- `tokenbucket/memory.go` (clock sample vs lock)
- `tokenbucket/repro_hunt_clock_last_backward_test.go` (new)
- `openspec/specs/std_go_tokenbucket_allow/spec.md`, `openspec/specs/std_go_tokenbucket_lua-eval/spec.md`
- `knowledge/devdocs/std_go_tokenbucket.md` (last-not-rewind gotcha)
- Fake Redis EVAL already applies `consumeOne`; it inherits persist-max. No SimpleRedis API change. No other packages.
