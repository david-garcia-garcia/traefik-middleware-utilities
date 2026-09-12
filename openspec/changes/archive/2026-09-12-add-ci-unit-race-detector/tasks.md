## 1. CI unit race

- [x] 1.1 Change `.github/workflows/ci.yml` job `test` Run Tests to `go test -race -short -timeout 10m -count=1 -v ./...`
- [x] 1.2 Leave job `e2e` without `-race` and without changing its 5m timeout or services

## 2. Catalog

- [x] 2.1 Update `knowledge/devdocs/std_go_test-suites.md` so unit CI is documented as passing `-race`
- [x] 2.2 Update README Tests so the unit suite sentence names `-race`

## 3. Validate

- [x] 3.1 Confirm dest concurrent pool tests still run under `-short` (no new `*_test.go`)
- [x] 3.2 Run `go test -short ./simpleredis/...` locally (without `-race` if gcc is missing). Record `localTests`
- [x] 3.3 Run `openspec validate --change add-ci-unit-race-detector --strict`
