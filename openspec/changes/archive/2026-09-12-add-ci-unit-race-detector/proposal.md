## Why

DestBranch Ubuntu CI compiles SimpleRedis without `go test -race`. The pool shares idle sockets, a closed flag, an in-use semaphore, and a capability cache across goroutines. Windows without gcc cannot run the detector, so CI is the only host that can check those accesses.

## What Changes

- Unit CI job `test` runs `go test -race -short -timeout 10m -count=1 -v ./...`.
- Go E2E stays without `-race` at its existing 5m cap (the non-race Ubuntu job).
- Keep four jobs. Do not add a fifth suite, Windows/macOS legs, fuzz, or pool runtime changes.
- Reuse dest concurrent unit tests as the detector canary. Do not add risk-05 token-conservation loops.
- Catalog unit `-race` on `std_go_ci_test-suites` and `knowledge/devdocs/std_go_test-suites.md` (README Tests follows that catalog).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_ci_test-suites`: Unit `test` SHALL pass `-race`, keep `-short` and `-count=1`, and SHALL use a timeout of at least 10m. `e2e` is not required to pass `-race`. A green race job is not proof of `inUseTurns` length / idle-cap logic.

## Impact

- `.github/workflows/ci.yml` job `test` Run Tests line.
- `openspec/specs/std_go_ci_test-suites/spec.md` after archive.
- `knowledge/devdocs/std_go_test-suites.md`; README Tests sentence that quotes unit `go test`.
- No SimpleRedis runtime change. No new `*_test.go` unless dest concurrent tests are missing (they are not).
