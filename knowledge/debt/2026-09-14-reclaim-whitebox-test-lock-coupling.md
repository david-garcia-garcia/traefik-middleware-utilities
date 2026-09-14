# White-box reclaim gap tests couple to mutex and slot internals

IssueKey: 2026-09-14-reclaim-ending-path-test-coverage
Size: large
Action: note

## Why this follow-up
`TestTable_OpenReplacesAMappedGoneIncarnation`, `TestTable_DropWaitsForAnInFlightTransition`, and `TestTable_ExpireLeavesAClaimedIncarnation` take `tab.mu` and construct `slot` state directly. That matches existing helpers (`mustSlot`, `readState`) and is the only reliable way to reach those branches. A parallel ticket (`2026-09-14-reclaim-bug-finished-race-lock-defer`) rewrites locking in `reclaim/table.go` to release `t.mu` with `defer` and may rename the mutex or slot fields. There is no textual conflict with `reclaim/table_gaps_test.go`, but a rename on that branch breaks these tests at merge.

## Why it was not taken
This run is test-only and must not edit `reclaim/table.go`. Coordinating with the sibling branch was forbidden. The sibling PR owns any rename; this PR should land the tests against current `master` names.

## Risks
If the locking PR merges second, CI on that branch (or the merge commit) fails in `./reclaim/` until the white-box tests are updated to the new field names. If this PR merges second, the same tests need a rebase onto the renamed internals before they compile.

## Context
Sibling adds only `reclaim/repro_finished_race_test.go` and `reclaim/repro_mutex_wedge_test.go` besides `table.go`. Coupling surface: `Table.mu`, `slot.state`, `slot.holders`, `slot.ready`, `slotGone` / `slotBusy` / `slotAsleep` / `slotAwake`.
