## 1. Package and production code

- [ ] 1.1 Add `traefikemulator/emulator.go` ported from bouncer `pkg/traefikemulator/emulator.go` (package comment, exports, stdlib-only imports unchanged in behavior)
- [ ] 1.2 Confirm module import path is `github.com/david-garcia-garcia/traefik-middleware-utilities/traefikemulator`

## 2. Port bouncer unit tests (six cases)

- [ ] 2.1 Add `traefikemulator/emulator_test.go` in package `traefikemulator` (not external test package)
- [ ] 2.2 Port `TestApply_CancelsPreviousGenerationBeforeNextNew`
- [ ] 2.3 Port `TestApply_RoutesShareOneContext`
- [ ] 2.4 Port `TestApply_OmittedRouteIsNotConstructed`
- [ ] 2.5 Port `TestApply_FailedNewIsAbsentAndSiblingStays`
- [ ] 2.6 Port `TestServe_HitsCurrentGenerationOnly`
- [ ] 2.7 Port `TestApply_SharedMiddlewareNameConstructsTwice`

## 3. Additional coverage (explore bar ~95%+)

- [ ] 3.1 Test duplicate route name in a single `Apply` returns an error and does not replace the first successful route
- [ ] 3.2 Test `Handler` and `Serve` return false for a route name never constructed or omitted after a later `Apply`
- [ ] 3.3 Test `Handler` and `Serve` return false after `Stop` for a previously constructed route
- [ ] 3.4 Test `Stop` cancels all contexts from the last generation (same assertion pattern as shared-context test)
- [ ] 3.5 Test empty `Apply` (or equivalent) leaves no servable routes when applicable to current map semantics
- [ ] 3.6 Test or document `New(nil)` panic (subtest with `recover` if implemented)
- [ ] 3.7 Run `go test -cover ./traefikemulator/` and add tests until statement coverage is ≥ ~95% or only the documented `New(nil)` panic branch remains uncovered

## 4. CI-shaped verification

- [ ] 4.1 Run `go test -short ./traefikemulator/...` (matches CI unit job scope for this package)
- [ ] 4.2 Run `go test -race -short ./traefikemulator/...`
- [ ] 4.3 Confirm no new `*_yaegi_*`, `*_e2e_*`, or Pester files were added for `traefikemulator`

## 5. Docs and catalog (implement / archive)

- [ ] 5.1 README: short section describing the test helper and a **Layout** row for `traefikemulator/`
- [ ] 5.2 `openspec validate 2026-09-24-add-traefikemulator --type change --strict`
