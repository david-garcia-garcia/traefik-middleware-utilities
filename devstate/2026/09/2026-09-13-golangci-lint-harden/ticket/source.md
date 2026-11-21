Run the repo's /opd-workflow skill to deliver ONE pull request that hardens the golangci-lint configuration and fixes every resulting violation.

Repo: d:\repositories\traefik-middleware-utilities (Go 1.21, Traefik plugin library, code must also run under the Yaegi interpreter). Base branch: master, currently at 2847a81. PR host is GitHub. Follow the repo's own conventions for branch naming, devstate/, openspec, and devdocs as /opd-workflow dictates.

This is a READABILITY AND GUARDRAIL PR ONLY. Do not change any runtime behavior. No logic changes, no refactors beyond what a linter demands.

Context: The requester already measured locally with golangci-lint v1.63.4 on a working branch, not master. RE-MEASURE everything on master before starting. Treat their numbers as expectations to verify, not facts.

Two local-only obstacles: untracked apm_modules/ and .agents/ contain Go eval fixtures that break typechecking (add them to issues.exclude-dirs); goimports/gofumpt need a diff binary that Windows lacks — either validate in CI or leave them out.

Task 1: enable the 17 linters that currently have ZERO violations (config-only): decorder, dogsled, durationcheck, godot, makezero, mirror, misspell, nakedret, nestif, nilerr, nilnesserr, perfsprint, reassign, recvcheck, tparallel, usestdlibvars, whitespace. Verify each is genuinely zero on master. If any now has hits, fix them if mechanical; if a hit reveals a real bug, STOP and report it rather than papering over it.

Task 2: enable linters that need fixes. Expected hits: gocritic with unnamedResult added (16 hits plus 3 default assignOp in tokenbucket/clock.go). CRITICAL: gocritic.enabled-checks ADDS to the default checker set. unnamedResult settings: checkExported: false (true means check exported functions ONLY — weaker; comment this in .golangci.yml). thelper 17 hits. revive 8 hits. dupword 1, prealloc 1, stylecheck 1, errname 1 (handshakeFailure was deleted in PR #67, expect 0). forcetypeassert 13 hits all in _test.go Yaegi interp.Eval — enable but exclude _test.go via issues.exclude-rules.
When naming results, do NOT introduce naked returns — keep returning explicitly. nakedret is enabled to guard that.

Task 3: name results on dial and borrow by hand. gocritic unnamedResult does NOT flag simpleredis.borrow or simpleredis.dial (*pooledConn, error, bool). Fix manually: named results so handshakeFailed appears in the signature. Do NOT reorder results to put error last (in-flight PRs). Note the deferred reorder in the PR body.

Task 4: enable errorlint with documented nolint suppressions. 11 hits, EVERY ONE IS DELIBERATE. Do NOT change them to errors.Is or errors.As. Add //nolint:errorlint // <specific reason>. Two real reasons: (1) Yaegi panics on errors.As against a package-local error struct; (2) Identity comparison is a WRITTEN SPEC REQUIREMENT for ErrPoolWait and errNotFromNew. If a site matches neither and errors.Is is genuinely correct and safe, convert and call it out.

Task 5: do NOT enable testpackage (54 hits; tests are in-package white-box). Record the rejected decision if the repo has a place for it.

Task 6: pin golangci-lint version in CI (.github/workflows/ci.yml currently version: latest). Pin to the exact v1.x version validated. v2 uses incompatible config schema.

Merge-order warning for PR body: in-flight branches 2026-09-13-simpleredis-panic-safe-release, 2026-09-13-simpleredis-lost-turn-recovery, 2026-09-13-simpleredis-desync-boundary-check, 2026-09-13-simpleredis-close-abandoned-socket, 2026-09-13-simpleredis-resilience-test-coverage.

Validation: full local test suite including Yaegi, golangci-lint run clean, measured CI green.
