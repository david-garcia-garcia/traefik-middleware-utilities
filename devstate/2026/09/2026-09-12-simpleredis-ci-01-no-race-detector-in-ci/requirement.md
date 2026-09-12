# Requirement
IssueKey: 2026-09-12-simpleredis-ci-01-no-race-detector-in-ci

## Problem
CI never runs `go test -race`. SimpleRedis is a hand-rolled pool (`idleConns` mutex, `closed` atomic, `inUseTurns` semaphore, `groupWrite` mutex). Dest Ubuntu jobs compile that package without the detector. Local Windows without gcc cannot run `-race` either, so CI is the only place that coverage can land.

## Current (code)
- `.github/workflows/ci.yml` job `test` (lines 27–38) — `ubuntu-latest`, `go test -short -timeout 2m -count=1 -v ./...`. No `-race`. No Redis/Dragonfly services.
- `.github/workflows/ci.yml` job `e2e` (lines 40–103) — `ubuntu-latest`, `go test -timeout 5m -count=1 -v ./...` without `-short`. No `-race`.
- `.github/workflows/ci.yml` jobs `lint` and `integration` — golangci-lint and Pester. No `go test -race`.
- `.github/workflows/` — no `windows` or `macos` jobs (`not found`).
- `openspec/specs/std_go_ci_test-suites/spec.md` — pins four jobs and the unit/e2e `go test` flags (`-short` vs not). Does not require `-race`.
- `knowledge/devdocs/std_go_test-suites.md` — catalogs lint / unit Go / Go E2E / Pester. Does not mention `-race`.
- `simpleredis/simpleredis.go:48-57` — `idleConnsMu`/`idleConns`, `closed atomic.Bool`, `inUseTurns`, `groupWriteMu`/`groupWrite`.
- `simpleredis/pool_test.go:27-49` — `TestConcurrentCommandsStayWithinPool` (8 goroutines × 20 Get).
- `simpleredis/simpleredis_test.go:45-71` — `TestCloseDuringInFlightCommandClosesSocketOnRelease` (one in-flight Get, then Close).
- `simpleredis/` — no token-conservation stress and no Close-under-traffic loop at the 60/200-round counts the ticket describes (`not found`).
- No `FuzzReadReply` (`not found`).

## Desired
1. At least one Ubuntu CI `go test` that compiles SimpleRedis runs with `-race`.
2. Raise that job’s timeout in the same change so `-race` plus Yaegi (and live jobs if they also take `-race`) do not fail the existing 2m/5m caps. Ticket example: `go test -race -timeout 10m -count=1 -v ./...`.
3. Keep `-count=1`.
4. Proof is a green CI run with `-race`. Land or reuse a concurrent test the detector would flag if the pool regressed (ticket names token-conservation stress and Close-under-traffic).
5. Do not treat a green `-race` run as proof of the `inUseTurns` length / idle-cap logic (channel length is not a data race).

## Affected
- `.github/workflows/ci.yml` (`test` and/or `e2e` `go test` lines, or a new Ubuntu `-race` job).
- `openspec/specs/std_go_ci_test-suites/spec.md` if the pinned `go test` invocation gains `-race` or a fifth job.
- `knowledge/devdocs/std_go_test-suites.md` if the catalog must name race coverage.
- `simpleredis/*_test.go` only if a detector-canary concurrent test is missing.

## Out of scope
- Fuzz target `FuzzReadReply`, fuzz corpus, CI `-fuzztime`.
- Adding `apm_modules/` to `.gitignore`.
- Other `simpleredisfixes2` findings (idle-cap / `inUseTurns` length, pipelining, context cancel, parser bounds).
- Adding Windows or macOS CI legs.
- Changing SimpleRedis runtime pool behaviour.

## Unknowns
- Which Ubuntu job takes `-race`: existing `test`, existing `e2e`, both, or a fifth job (ticket: one Ubuntu leg; dest has no windows/macos to keep plain).
- Whether dest `TestConcurrentCommandsStayWithinPool` and `TestCloseDuringInFlightCommandClosesSocketOnRelease` are enough canaries, or the ticket’s token-conservation / Close-under-traffic stresses must be added.
- Wall time of Yaegi plus `-race` versus 10m on the unit job, and of live engines plus `-race` versus 10m on e2e.

## Tensions
- Ticket cites `.github/workflows/ci.yml:67` as `go test -timeout 2m -count=1 -v ./...`. Dest line 67 is e2e healthcheck prose; unit Run Tests is line 38 (`-short -timeout 2m`); e2e Run Tests is line 100 (`-timeout 5m`). Neither has `-race`.
- Ticket: windows/macos can stay non-race. Dest has only Ubuntu jobs.
- Ticket: raise the one test step to 10m. Dest already split 2m unit / 5m e2e.
- Ticket proof: land detector-canary stresses. Dest has concurrent Get and one in-flight Close test, not the auditor’s 60/200-round loops.
- Ticket says `-race` will not flag `len(inUseTurns)` under `idleConnsMu` (bug-03). That stays out of scope; a green race job must not be read as validating that logic.
