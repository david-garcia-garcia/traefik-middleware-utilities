# Requirement
IssueKey: 2026-09-12-simpleredis-build-01-test-binary-does-not-build

## Problem
`simpleredis/interpretedcost_test.go` calls `writeGopathFile` at four sites. If that helper is missing, `go test ./simpleredis/` fails to compile and no test in the package runs. Committing those tests without the helper would turn CI `go test ./...` red and hide every other failure in the package.

## Current (code)
- `simpleredis/interpretedcost_test.go` (tracked on dest `origin/master` @ `0159cfc`): `writeGopathFile` at `:21`, `:190`, `:273`, `:380`.
- `simpleredis/yaegi_test.go` `writeGopathFile` (`:138`–`:148`): `func writeGopathFile(t testing.TB, goPath, pkg, name, src string)` writes `GOPATH/src/<pkg>/<name>`. `writeGopathClientprobe` delegates to it.
- `simpleredis/bench_test.go` (tracked on dest): no `writeGopathFile` call sites.
- Dest worktree: `go test -c -o NUL ./simpleredis/` exit 0.
- Sibling copies (not shared): `tokenbucket/limiter_yaegi_test.go:106`, `windowcounter/limiter_yaegi_test.go:108`, `reclaim/yaegi_test.go:98`.
- CI on dest: `.github/workflows/ci.yml:38` (`go test -short ... ./...`) and `:100` (`go test ... ./...`). Ticket `:67` is env vars on this dest, not the test step.
- Caller working tree (do not copy): untracked `simpleredis/interpretedcost_test.go` and `simpleredis/bench_test.go`; `simpleredis/yaegi_test.go` has `writeGopathSimpleredis` and `writeGopathClientprobe` only — no `writeGopathFile`. Content hashes differ from dest for all three files.

## Desired
1. Dest `simpleredis` test binary compiles. Do not land a red test binary.
2. The fix for a missing `writeGopathFile` is that helper in the `simpleredis` test files (signature matching the four call sites). Dest already has it.
3. Commit `interpretedcost_test.go` and `bench_test.go` only if they belong on dest. Dest already tracks both. Do not replace dest files with the caller’s untracked copies.

## Affected
- `simpleredis/yaegi_test.go` (helper)
- `simpleredis/interpretedcost_test.go` (call sites)
- `simpleredis/bench_test.go` (named by the ticket; dest already tracks it)

## Out of scope
- Every other `simpleredisfixes2/` finding (bug-01, bug-02, ci-01, and the rest).
- Factoring `writeGopathFile` into a shared internal test-support package across `simpleredis`, `tokenbucket`, `windowcounter`, and `reclaim`.
- Adding `apm_modules/` to `.gitignore`.
- Copying or merging the caller’s untracked working-tree files onto dest.
- Changing CI workflow layout.

## Unknowns
- Runtime of dest Yaegi tests (`TestYaegi_*`) was not run in prepare; only compile (`go test -c`) was measured.
- Why the caller’s untracked copies diverge from dest (caller HEAD `cc38380`, eight commits behind `origin/master`).

## Tensions
- Ticket says dest HEAD is clean and the two test files are untracked. Dest `origin/master` already tracks them and defines `writeGopathFile`. The compile break the ticket measured is the caller’s dirty tree at an older HEAD, not dest.
- Ticket “Better: factor the helper into one internal package” is extra versus “define the missing helper in `simpleredis`”. Dest already has the package-local helper.
- Ticket “close `apm_modules/` gitignore together” is `ci-01`, not this finding.
