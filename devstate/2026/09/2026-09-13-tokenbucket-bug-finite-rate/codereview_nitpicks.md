# Nitpicks

1. [hard] Name for the scope — `tokenbucket/repro_hunt_inf_rate_nan_test.go:13` — `func TestRepro_InfRateElapsedZeroNaNFailOpen` names dest elapsed=0 Inf*0=NaN fail-open; the body only runs `NewMemory`/`NewRedis` and asserts `errRate` for ±Inf
   → Rename to `TestRepro_InfRateRejected` (same job as `TestRepro_NaNRateRejected`)
   Status: done
   Argument: renamed to `TestRepro_InfRateRejected` in `tokenbucket/repro_inf_rate_test.go`.
