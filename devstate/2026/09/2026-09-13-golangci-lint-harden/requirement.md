# Requirement
IssueKey: 2026-09-13-golangci-lint-harden

## Problem
Dest lint is a short `.golangci.yml` plus CI `version: latest`. Many mechanical linters are off. Named results, helper `t.Helper()`, identity error compares, and Yaegi `Eval` type asserts are unguarded. This PR is readability and guardrails only: enable the listed linters, fix every resulting violation without changing runtime behavior, pin a validated v1.x golangci-lint.

## Current (code)
- `.golangci.yml` enables only `asciicheck`, `exhaustive`, `gochecknoinits`, `goconst`, `gofmt`, `gosec`, `predeclared`, `unconvert`, `unparam`, `wastedassign`. No `issues.exclude-dirs`, no `issues.exclude-rules`, no `gocritic` / `thelper` / `revive` / `errorlint` / `forcetypeassert` / the Task 1 set.
- `.github/workflows/ci.yml` job `lint` uses `golangci/golangci-lint-action` with `version: latest`. `openspec/specs/std_go_ci_test-suites/spec.md` requires a `lint` job, not a pinned version. `knowledge/devdocs/std_go_test-suites.md` points at those two files.
- `simpleredis/pool.go` `borrow` and `dial` return `(*pooledConn, error, bool)`. Comments name the bool `handshakeFailed`; the results are unnamed. `simpleredis/commands_exec.go` already unpacks `conn, err, handshakeFailed := sr.borrow(ctx)`.
- `type handshakeFailure` is `not found` on dest (`2847a81`, PR #67). `errname` on that name is expected 0.
- `tokenbucket/clock.go` `consumeOne` uses `tokens = tokens + …`, `tokens = tokens - 1`, `tokens = tokens + 1` (gocritic `assignOp` shape). Results there are already named.
- `simpleredis/commands_exec.go` `isCommandTimeout` / `isUnreachable` compare `err == errTimeout` / `err == errUnreachable`. `simpleredis/resp.go` `isDirtyProtocolError` compares `err == errIssue || err == errUnsupportedReply`. `openspec/specs/std_go_simpleredis_tcp-session/spec.md` requires identity (or a pool-wait check before unreachable) so `errors.Is` against unreachable must not retry `ErrPoolWait`. `errNotFromNew` is `fmt.Errorf("%w", ErrUnreachable)` in `simpleredis/simpleredis.go`.
- Yaegi helpers force-assert `evaluated.Interface().(string)` in `simpleredis/yaegi_test.go`, `simpleredis/yaegi_errorpath_test.go`, `simpleredis/interpretedcost_test.go`, `tokenbucket/limiter_yaegi_test.go`, `windowcounter/limiter_yaegi_test.go`, `reclaim/yaegi_test.go`, `backendbackoff/gate_yaegi_test.go`.
- Tests are in-package: no `package *_test` in this tree.
- `apm_modules/` and `.agents/` Go eval fixtures are `not found` on dest (untracked in the caller checkout only).
- goimports / gofumpt are `not found` in `.golangci.yml`.

## Desired
1. Re-measure every cited linter on dest `master` (`2847a81`) before enabling. Requester counts (golangci-lint v1.63.4 on a working branch) are expectations, not facts.
2. Enable the 17 zero-hit linters when they are genuinely zero on dest: `decorder`, `dogsled`, `durationcheck`, `godot`, `makezero`, `mirror`, `misspell`, `nakedret`, `nestif`, `nilerr`, `nilnesserr`, `perfsprint`, `reassign`, `recvcheck`, `tparallel`, `usestdlibvars`, `whitespace`. Mechanical hits: fix. A hit that is a real bug: stop and report; do not paper over.
3. Enable and fix: `gocritic` with `unnamedResult` added (`enabled-checks` adds to the default set; `checkExported: false` — comment that `true` checks exported functions only and is weaker). Expected on the requester branch: 16 unnamedResult plus 3 `assignOp` in `tokenbucket/clock.go`. `thelper`, `revive`, `dupword`, `prealloc`, `stylecheck`, `errname` (expect 0). Named results must keep explicit `return` (`nakedret` stays on).
4. Name `borrow` / `dial` results by hand so `handshakeFailed` is in the signature. Do not reorder to put `error` last (in-flight SimpleRedis PRs). Note the deferred reorder in the PR body.
5. Enable `forcetypeassert` and exclude `_test.go` via `issues.exclude-rules` (Yaegi `interp.Eval` asserts).
6. Enable `errorlint`. Do not rewrite the deliberate `==` sites to `errors.Is` / `errors.As`. Add `//nolint:errorlint // <specific reason>`. Two reasons: Yaegi panics on `errors.As` against a package-local error struct; identity compare is a spec requirement for `ErrPoolWait` / `errNotFromNew`. Convert only if a site matches neither and `errors.Is` is safe; call that out.
7. Do not enable `testpackage` (in-package white-box tests). Record that rejection in this file; later phases may copy it into usage docs.
8. Add `issues.exclude-dirs` for `apm_modules` and `.agents` so local untracked fixtures do not break typechecking.
9. goimports / gofumpt: enable in CI only, or leave out (Windows lacks a `diff` binary).
10. Pin CI golangci-lint to the exact v1.x that was validated. Do not use v2 (incompatible schema). Do not change runtime behavior.

## Affected
- `.golangci.yml`
- `.github/workflows/ci.yml` (`version:` only)
- Mechanical lint fixes in dest Go (named results, `t.Helper()`, `assignOp`, `//nolint:errorlint`, revive/style/prealloc/dupword as measured)
- `simpleredis/pool.go` `borrow` / `dial` result names
- Possibly `openspec/specs/std_go_ci_test-suites/spec.md` and `knowledge/devdocs/std_go_test-suites.md` if propose adds a pin requirement

## Out of scope
- Any runtime behavior change, logic change, or refactor the linter does not demand
- Reordering `borrow` / `dial` results to put `error` last
- Enabling `testpackage`
- Converting spec/Yaegi identity compares to `errors.Is` / `errors.As`
- Enabling goimports / gofumpt on Windows local runs
- Product work on in-flight SimpleRedis branches (`2026-09-13-simpleredis-panic-safe-release`, `2026-09-13-simpleredis-lost-turn-recovery`, `2026-09-13-simpleredis-desync-boundary-check`, `2026-09-13-simpleredis-close-abandoned-socket`, `2026-09-13-simpleredis-resilience-test-coverage`)

## Unknowns
- Hit counts on dest `master` (not re-measured this phase; requester numbers are from another branch).
- Exact v1.x to pin (requester used v1.63.4; implement must validate, then pin that validated version).
- Which of the 17 “zero” linters still have zero hits on dest.
- Locations of `revive` (8), `dupword` (1), `prealloc` (1), `stylecheck` (1), `thelper` (17) until re-measure.
- Whether any `errorlint` site is neither Yaegi nor the identity spec (would convert, not nolint).
- goimports / gofumpt: CI-only vs omitted.
- Whether propose pins the lint version in `std_go_ci_test-suites`.

## Tensions
- Requester measured v1.63.4 off dest; dest is `2847a81`. Counts can disagree; dest wins after re-measure.
- `gocritic.enabled-checks` adds to defaults; omitting that fact would drop default checkers (including `assignOp`).
- `unnamedResult.checkExported: false` is the stronger setting; `true` would skip unexported `borrow` / `dial` anyway, which this ticket still names by hand.
- `do` in `simpleredis/resp.go` already returns `([][]byte, bool, error)` (error last). `borrow` / `dial` stay `(conn, error, bool)` until a later reorder.
- Identity `err == errUnreachable` is what keeps `ErrPoolWait` / `errNotFromNew` from being retried; `errors.Is` would be a behavior change.
- Merge order: the five in-flight SimpleRedis branches listed above may touch the same signatures and `==` sites.
