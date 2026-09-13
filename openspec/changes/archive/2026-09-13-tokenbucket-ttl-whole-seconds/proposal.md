## Why

`NewMemory` / `NewRedis` accept a `ttl` Redis cannot represent (1500ms). Memory expires at `now.Add(ttl)`; Redis EVAL ARGV is `int64(ttl / time.Second)` so EXPIRE is 1. The stores disagree. Redis integer-second EXPIRE is the Traefik contract.

## What Changes

- **BREAKING** (constructor): `validateClock` requires `ttl` to be a whole number of seconds and still `>= 1s`. Same `errTTL` sentinel. `New(1500ms)` returns that error.
- Tests first: dest-failing proof that NewMemory/NewRedis accept 1500ms, Redis ARGV ttl is `"1"`, Memory is not expired at +1200ms; then reshape so New rejects 1500ms and still accepts 2s.
- Do not PEXPIRE. Do not rewrite Lua. Do not silently floor Memory to 1s while New(1500ms) succeeds.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: construction SHALL fail when `ttl` is not a whole number of seconds (still `>= 1s`), not only when `ttl < 1s`.
- `std_go_tokenbucket_lua-eval`: Memory and Redis SHALL agree because New rejects a ttl Redis cannot pass as EXPIRE seconds. MUST NOT switch to PEXPIRE or floor Memory while New succeeds.

## Impact

- `tokenbucket/clock.go` (`validateClock`, `errTTL` text)
- `tokenbucket/ttl_truncation_test.go` (adapted from the caller repro; dest `Allow(ctx, key)` signature)
- `openspec/specs/std_go_tokenbucket_allow/spec.md` and `std_go_tokenbucket_lua-eval/spec.md` after archive
- `knowledge/devdocs/std_go_tokenbucket.md` gotcha (`ttl < 1s` → whole seconds)
