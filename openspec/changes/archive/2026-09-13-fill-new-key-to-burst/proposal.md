## Why

A new token-bucket key is supposed to start at `burst` then consume 1. Dest seeds missing state as `tokens=0` `last=0` and refills from elapsed since Unix epoch, so the first consume is empty at `Unix(0,0)` and short of burst when `burst` is huge (e.g. `1e12`).

## What Changes

- Missing state is a full bucket, then consume 1, on both Memory and Redis.
- Lua empty `HGETALL` (`#rl_source ~= 4`) sets `tokens = burst` and `last = t` (elapsed 0), then the existing refill/consume. Keep the Traefik Labs MIT notice. Do not rely on `last=0` plus elapsed-from-epoch.
- Memory: a new `memEntry` (including after TTL delete) starts `tokens = burst`, `last = nowMicro`, not zeros. Same formulas after that.
- Fake Redis missing hash seeds like Lua so `consumeOne` stays consume math and Memory/Redis agreement still holds.
- Tests first: dest-adapted `TestRepro_NewKeyFillsToBurstAtEpoch` (`epoch_clock` burst 5; `huge_burst_elapsed_below_burst` burst `1e12`) MUST fail on dest, then PASS after the fill. Redis/fake proof under `-short` (no live engines required).
- Do not import `golang.org/x/time/rate`. Do not change wait mapping, NaN rate, ttl seconds, last rewind, or the `Allow(ctx, key)` signature.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: Idle / new / TTL-expired key SHALL start at `burst` then consume 1 even when `now` is Unix epoch or `burst` exceeds elapsed-from-epoch times rate. MUST NOT import `golang.org/x/time/rate`.
- `std_go_tokenbucket_lua-eval`: Empty hash SHALL seed `tokens = burst` and `last = t` inside Eval (intentional delta from Traefik Redis idle). Memory and Redis SHALL still agree. MIT notice kept.

## Impact

- `tokenbucket/lua.go` empty-hash seed; `tokenbucket/memory.go` new-entry seed; `tokenbucket/fake_redis_test.go` missing-hash seed.
- New failing-then-green tests in `tokenbucket/repro_epoch_idle_burst_test.go`.
- Usage packet `knowledge/devdocs/std_go_tokenbucket.md` new-key fill gotcha (devdocs-impact).
- Redis hash fields `last` / `tokens` still exist; first write after a miss stores `last = now` and `tokens = burst - 1` instead of epoch-elapsed refill.
