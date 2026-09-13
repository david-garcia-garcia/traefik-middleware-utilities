# Non-finite tokenbucket rate is accepted (NaN / Inf)

THIS BUG ONLY. Do not change Allow wait mapping, Eval wait, ttl, Lua fill, or last rewind.

## Problem
validateClock only rejects rate <= 0. math.NaN() <= 0 is false; +Inf > 0 is true. NewMemory/NewRedis accept NaN and Inf. NaN rate: Allow always true. +Inf: Inf*0=NaN on second Allow at elapsed=0, fail-open with maxDelay=0.
This package has no unlimited-rate constructor. Passthrough is skipping New.

## Agreed how
validateClock rejects any non-finite rate as errRate: rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0). Both constructors already go through this. Do not special-case Allow or consumeOne for NaN tokens. Do not treat +Inf as unlimited.

Source: tokenbucket/BUGS.md item 3.

## Tests first (hard)
CREATE tests, confirm FAIL, then fix, then PASS.
Copy/adapt:
- tokenbucket/repro_nan_rate_test.go — TestRepro_NaNRateRejected (NewMemory/NewRedis NaN), TestRepro_NaNRateAllowFailOpen. Do NOT keep TestRepro_NaNRateInfAllow as "Inf should always allow" — Inf must now be rejected at New.
- tokenbucket/repro_hunt_inf_rate_nan_test.go — +Inf New + second Allow; after the fix New must error (adjust assertion to New rejects Inf).
