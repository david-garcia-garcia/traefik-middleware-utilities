## Why

`validateClock` only rejects `rate <= 0`. `NaN <= 0` is false and `+Inf > 0` is true, so `NewMemory` and `NewRedis` accept those rates and `Allow` fail-opens. This package has no unlimited-rate constructor; passthrough is skipping `New`.

## What Changes

- `validateClock` rejects any non-finite rate as `errRate`: `rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0)`.
- Land unit tests that fail on dest for NaN and +Inf at `New`, then pass after the gate. Do not keep an Inf-always-allow test. Do not special-case `Allow` or `consumeOne`. Do not treat +Inf as unlimited.
- Widen `std_go_tokenbucket_allow` construction fail from `rate <= 0` to non-finite. Update the usage gotcha.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: construction SHALL fail when rate is NaN or infinite (`errRate`), not only when `rate <= 0`. Both `NewMemory` and `NewRedis` already share `validateClock`.

## Impact

- `tokenbucket/clock.go` — `validateClock` finite-rate gate; import `math`.
- `tokenbucket/repro_nan_rate_test.go` and `tokenbucket/repro_hunt_inf_rate_nan_test.go` — fail-then-pass constructor tests; dest `Allow` three-value signature.
- `openspec/specs/std_go_tokenbucket_allow/spec.md` after archive.
- `knowledge/devdocs/std_go_tokenbucket.md` — gotcha: non-finite rate fails New.
