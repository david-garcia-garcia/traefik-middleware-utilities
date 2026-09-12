## 1. CI unit race

- [ ] 1.1 Change `.github/workflows/ci.yml` job `test` Run Tests to `go test -race -short -timeout 10m -count=1 -v ./...`
- [ ] 1.2 Leave job `e2e` without `-race` and without changing its 5m timeout or services

## 2. Catalog

- [ ] 2.1 Update `knowledge/devdocs/std_go_test-suites.md` so unit CI is documented as passing `-race`
- [ ] 2.2 Update README Tests so the unit suite sentence names `-race`

## 3. Validate

- [ ] 3.1 Confirm dest concurrent pool tests still run under `-short` (no new `*_test.go`)
- [ ] 3.2 Run `go test -short ./simpleredis/...` locally (without `-race` if gcc is missing). Record `localTests`
- [ ] 3.3 Run `openspec validate --change add-ci-unit-race-detector --strict`
