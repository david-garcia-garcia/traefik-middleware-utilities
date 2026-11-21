# Requirement
IssueKey: 2026-09-14-reclaim-ending-path-test-coverage

## Problem
The `reclaim` package on `master` reports ~94.4% statement coverage with nine uncovered blocks. Two documented behaviours in `knowledge/devdocs/std_go_reclaim.md` (Reset Sleep-panic orphan skip; Reset unmaps before EnforceCloseBeforeOpen Close) have no tests. The ticket lands caller-authored gap tests at 100% coverage without changing production code.

## Current (code)
- `reclaim/table.go` — nine blocks never executed by existing package tests: `waitCtx` when `ctx.Done() != nil` (`reclaim/table.go` L144–150); `waitCtx` polling-loop fast path before ticker select (`reclaim/table.go` L154–159); `Open` `case slotGone` with key still mapped (`reclaim/table.go` L322–327); `drop` busy-wait `for incarnation.state == slotBusy` (`reclaim/table.go` L437–443); `expire` early return when state/holders no longer asleep (`reclaim/table.go` L506–508); `Reset` Sleep-panic path that logs panic and still `dispose` without orphan (`reclaim/table.go` L558–564); `Reset` leaving busy slots to owning goroutines (`reclaim/table.go` L567–570).
- `reclaim/` tests on `master` — no `table_gaps_test.go`; coverage gaps above remain (caller measured 94.4%).
- `knowledge/devdocs/std_go_reclaim.md` — documents Reset Sleep-panic and Reset unmap-before-enforce (`knowledge/devdocs/std_go_reclaim.md` L95–96 area); not asserted in tests today.
- `D:/repositories/traefik-middleware-utilities/reclaim/table_gaps_test.go` — exists only in main checkout (untracked); not on branch/worktree yet.

## Desired
- Add `reclaim/table_gaps_test.go` (verbatim from caller reference) during implement only; `reclaim/table.go` byte-identical to `origin/master`.
- Tests cover all nine blocks including direct `waitCtx` calls, white-box `tab.mu`/`slot` shapes for Open slotGone, drop busy-wait, and expire race, plus Reset Sleep-panic and EnforceCloseBeforeOpen unmap behaviour per devdocs.
- Acceptance: `go test -count=1 -timeout 10m ./reclaim/` green; Docker `golang:1.25` race run green; statement coverage 100.0% with zero uncovered blocks in `reclaim`.

## Affected
- `reclaim/table_gaps_test.go` (new, implement phase)
- `devstate/` bus (prepare through pullrequest)
- Possible spec/devdocs deltas when documented contracts gain tests (propose/devdocsimpact)

## Out of scope
- Any edit to `reclaim/table.go` or production behaviour changes.
- Deleting or commenting unreachable defensive paths (`waitCtx` Done branch, drop busy-wait, redundant poll fast path) — deferred to later `issues.md` / `knowledge/debt/` notes, not this ticket.
- Landing `table_gaps_test.go` during prepare (only empty start commit + bus).
- Coordinating with parallel branch `2026-09-14-reclaim-bug-finished-race-lock-defer`.

## Unknowns
- Exact statement/block coverage on `origin/master` without running `go test -cover` in prepare (caller asserts 94.4% / nine blocks).
- Whether parallel locking refactor renames `tab.mu`, `slot`, or internal fields before merge (could break white-box tests).
- Whether CI requires additional coverage gates beyond package tests.

## Tensions
- White-box tests couple to `tab.mu` and unexported `slot` state while a sibling ticket may refactor locking in `reclaim/table.go` — merge-order risk, not a prepare resolution.
- Caller analysis treats items 1, 2, 4 as dead/redundant code but forbids deletion here; explore/codereview may still propose debt-file notes without changing scope of this ticket.
