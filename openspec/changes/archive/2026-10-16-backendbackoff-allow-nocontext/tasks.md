## 1. API

- [x] 1.1 Change `Gate.Allow` to `Allow(key string)` and remove `ctx.Err()` pre-check in `backendbackoff/allow.go`
- [x] 1.2 Remove unused `context` import from `allow.go` if no longer referenced

## 2. Tests and benchmarks

- [x] 2.1 Delete `TestAllow_CanceledContext` from `gate_test.go`
- [x] 2.2 Update remaining `gate_test.go` call sites to `Allow("k")` (or equivalent keys)
- [x] 2.3 Update `gate_yaegi_test.go` probe to call `Allow` without context
- [x] 2.4 Update `bench_test.go` to benchmark `Allow(key)` only
- [x] 2.5 Run `go test -short ./backendbackoff/...` and fix any compile or behavior regressions

## 3. Documentation

- [x] 3.1 Update `knowledge/devdocs/std_go_backendbackoff.md`: `Allow(key)`, cancellation before call, revised pattern snippet
- [x] 3.2 Optional: align `README.md` example; if done, append row to `devstate/deviations.md`

## 4. Verification

- [x] 4.1 Confirm archived spec path: delta under `openspec/changes/backendbackoff-allow-nocontext/specs/std_go_backendbackoff_allow/` matches implement behavior before archive phase
