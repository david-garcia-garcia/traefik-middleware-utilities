## 1. OpenWithHooks and Open wrapper

- [ ] 1.1 Change `put` to `create func() (any, Hooks, error)`, drop its `hooks` parameter, capture returned hooks, pass them to `publishPut`
- [ ] 1.2 Add `OpenWithHooks` with the dest `Open` lookup loop and the same nil table / logger / create checks and error strings
- [ ] 1.3 Turn `Open` into a wrapper: nil-create branch reports table then logger then create; else wrap create + fixed hooks and call `OpenWithHooks`

## 2. OpenTyped

- [ ] 2.1 Add `reclaim/opentyped.go` with `OpenTyped[T any]` calling `OpenWithHooks`, mismatch error `reclaim: open %q: want %T, got %T`, Yaegi call-expression constraint in the doc comment

## 3. Tests

- [ ] 3.1 OpenWithHooks: `EnforceCloseBeforeOpen` returned from create is honoured; a later Open binds without re-running create
- [ ] 3.2 OpenTyped: typed return, singleton identity, type-mismatch error path
- [ ] 3.3 Regression: Open nil-argument precedence unchanged, including nil table + nil create and nil logger + nil create
- [ ] 3.4 Yaegi: call-expression `OpenTyped` case in `reclaim/yaegi_test.go` (skip under `-race` like siblings)

## 4. Specs and usage

- [ ] 4.1 Keep the change delta on `std_go_reclaim_value-lifecycle` aligned with the apply
- [ ] 4.2 Update `knowledge/devdocs/std_go_reclaim.md` Language Open/Hooks and the pattern snippet to show `OpenTyped` / `OpenWithHooks`

## 5. Gates

- [ ] 5.1 `go test -count=1 ./reclaim/`
- [ ] 5.2 Repo lint (`.golangci.yml`)
- [ ] 5.3 Run or skip-with-reason `Test-Integration.ps1` and `e2e/` as the repo requires; record outcomes
