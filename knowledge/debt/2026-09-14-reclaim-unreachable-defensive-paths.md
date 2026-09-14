# Unreachable reclaim defensive paths: delete or comment

IssueKey: 2026-09-14-reclaim-ending-path-test-coverage
Size: large
Action: note

## Why this follow-up
Three defensive stretches in `reclaim/table.go` are either unreachable from the table or redundant with the statement below them. This coverage ticket pins them with tests instead of removing them.

1. `waitCtx`'s `ctx.Done() != nil` select (`reclaim/table.go` around L144–150). `watch` is the only caller, and `dropWhenDone` starts `watch` only when `ctx.Done()` is nil. The three statement blocks stay at count 0 until a test calls `waitCtx` directly.
2. `drop`'s `for incarnation.state == slotBusy` wait (`reclaim/table.go` around L437–443). A busy slot only ever has holders that have not bound yet, so no pending drop meets one under an ordinary schedule.
3. `waitCtx`'s non-blocking `select` on `finished` at the top of the polling loop (`reclaim/table.go` around L154–159). It duplicates the blocking select below it. On `master` the block already has count 2; it only wins when both channels are ready and the select picks the ticker.

A later change should either delete them (if the author is sure no future caller needs the defense) or leave an explicit comment that they are unreachable / redundant, so the next coverage hunt does not rediscover them as gaps.

## Why it was not taken
The requirement lists deletion and commenting as out of scope. Unattended take is only small rows on files this run created. `table.go` is production structure with many callers; this change is test-only and must stay byte-identical to `master` on that file.

## Risks
Leaving the statements looks like live recovery paths. A later edit can "simplify" `dropWhenDone` into always calling `watch`, or can assume `drop` never sees `slotBusy`, and then the untested-from-the-table branches become load-bearing without anyone noticing they were previously dead. The new tests call `waitCtx` and `drop` directly, so a deletion would fail those tests; a silent comment-only follow-up still needs a human choice.

## Context
Measured on `origin/master` e9ac73c: `reclaim` 94.4% of statements, nine profile rows with count 0. Items 1 and 2 above are five of those rows. Item 3 is not a zero-count row. Tests that pin them live in `reclaim/table_gaps_test.go` (`TestWaitCtx_DoneChannelBranch`, `TestWaitCtx_PollingSeesAnEndedIncarnationFirst`, `TestTable_DropWaitsForAnInFlightTransition`) once this change lands.
