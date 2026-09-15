## 1. OpenWithHooks and Open wrapper

- [x] 1.1 Change `put` to `create func() (any, Hooks, error)`, drop its `hooks` parameter, capture returned hooks, pass them to `publishPut`
- [x] 1.2 Add `OpenWithHooks` with the dest `Open` lookup loop and the same nil table / logger / create checks and error strings
- [x] 1.3 Turn `Open` into a wrapper: nil-create branch reports table then logger then create; else wrap create + fixed hooks and call `OpenWithHooks`

## 2. OpenTyped

- [x] 2.1 Add `reclaim/opentyped.go` with `OpenTyped[T any]` calling `OpenWithHooks`, mismatch error `reclaim: open %q: want %T, got %T`, Yaegi call-expression constraint in the doc comment

## 3. Tests

- [x] 3.1 OpenWithHooks: `EnforceCloseBeforeOpen` returned from create is honoured; a later Open binds without re-running create
- [x] 3.2 OpenTyped: typed return, singleton identity, type-mismatch error path
- [x] 3.3 Regression: Open nil-argument precedence unchanged, including nil table + nil create and nil logger + nil create
- [x] 3.4 Yaegi: call-expression `OpenTyped` case in `reclaim/yaegi_test.go` (skip under `-race` like siblings)

## 4. Specs and usage

- [x] 4.1 Keep the change delta on `std_go_reclaim_value-lifecycle` aligned with the apply
- [x] 4.2 Update `knowledge/devdocs/std_go_reclaim.md` Language Open/Hooks and the pattern snippet to show `OpenTyped` / `OpenWithHooks`

## 5. Gates

- [x] 5.1 `go test -count=1 ./reclaim/`
- [x] 5.2 Repo lint (`.golangci.yml`)
- [x] 5.3 `Test-Integration.ps1 -Suite reclaim` failed locally: Docker bind 0.0.0.0:8000 already allocated. `e2e/reclaimprobe` unchanged (stays on `Open`). CI Integration Tests job is the measured Pester run.
