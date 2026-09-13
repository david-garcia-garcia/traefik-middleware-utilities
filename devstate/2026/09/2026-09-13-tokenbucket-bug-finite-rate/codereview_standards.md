# Standards

1. [hard] Name for the scope — `tokenbucket/repro_hunt_inf_rate_nan_test.go:1` — file is `repro_hunt_inf_rate_nan_test.go`; body only asserts New rejects ±Inf (`TestRepro_InfRateElapsedZeroNaNFailOpen`). `hunt` and `nan` hide that job
   → Rename the file to `repro_inf_rate_test.go`
   Status: done
   Argument: renamed to `tokenbucket/repro_inf_rate_test.go` (`6986195` apply; this rename after review).
2. [hard] Leave a trail — `tokenbucket/clock.go:11` — `errRate` still says `rate must be greater than 0`; `validateClock` now returns it for +Inf, which is greater than 0
   → Change the `errRate` text to say rate must be finite and greater than 0 (keep the same sentinel for `errors.Is`)
   Status: skipped
   Argument: public Error() text; explore assumed keep the string; ticket named `errRate` not copy change. `errors.Is` still matches.
