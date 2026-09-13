## 1. Failing tests first

- [x] 1.1 Add `tokenbucket/repro_nan_rate_test.go` with `TestRepro_NaNRateRejected` (NewMemory/NewRedis NaN → `errRate`, nil limiter) and `TestRepro_NaNRateAllowFailOpen` (Skip when New rejects NaN). Dest `Allow` is `(bool, time.Duration, error)`. Do not keep Inf-always-allow.
- [x] 1.2 Add `tokenbucket/repro_hunt_inf_rate_nan_test.go` asserting New rejects +Inf (and `-Inf`) as `errRate`. After the fix New MUST error; do not assert second Allow fail-open as the post-fix contract.
- [x] 1.3 Run `go test -short -count=1 -timeout 60s -run 'TestRepro_NaNRate|TestRepro_InfRate' ./tokenbucket` and confirm FAIL on dest (NaN and +Inf). Record the output.

## 2. Constructor gate

- [x] 2.1 In `validateClock`, reject `rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0)` as `errRate`. Import `math`. Do not special-case `Allow` or `consumeOne`. Do not treat +Inf as unlimited.
- [x] 2.2 Re-run the same `go test -short` command and confirm PASS. Run `go test -short ./tokenbucket` so existing tests stay green.

## 3. Usage

- [x] 3.1 Update `knowledge/devdocs/std_go_tokenbucket.md` gotcha: non-finite rate fails New; passthrough is skipping construction.
