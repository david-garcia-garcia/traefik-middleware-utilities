# Requirement
IssueKey: 2026-09-13-reclaim-bug-nildone-watcher-leak

## Problem
A holder whose `Done()` is nil cannot use `context.AfterFunc`, so `dropWhenDone` starts `go t.watch`, and `watch` calls `waitCtx`, which returns only when `ctx.Err()` is set. `context.Background()` never sets it. After that key's incarnation has ended, the watcher keeps polling at 20 ms for the process lifetime, retains `key` and the `*slot` (and the stored value), and never calls `drop` because `Err()` stays nil. Spec `std_go_reclaim_context-lease` already requires those goroutines to exit once holders are Done **and** the incarnation has ended.

## Current (code)
- `reclaim/table.go` `requireContext` — panics only when `ctx == nil`. Comment: Background is still accepted.
- `reclaim/table.go` `waitCtx` — if `Done() != nil`, wait on it; else `for ctx.Err() == nil { time.Sleep(20 * time.Millisecond) }`. No incarnation-end exit.
- `reclaim/table.go` `dropWhenDone` — `Done() != nil` → `context.AfterFunc` → `drop`; else `go t.watch`.
- `reclaim/table.go` `watch` — `waitCtx(ctx)` then always `t.drop(key, incarnation)`.
- `reclaim/table.go` `slotGone` comment — the key is unmapped; nothing can reach the slot except a watcher that predates the claim.
- `reclaim/table.go` `endBusySlot`, `unmapAfterClose`, `expire` (non-enforce sets `slotGone` then dispose), `Reset` (unmaps all, sets awake/asleep to `slotGone`, sleeps/disposes) — none stop `waitCtx`.
- `reclaim/table.go` `drop` — first line `incarnation.holders--`; if state is not `slotAwake` it returns after that decrement. Sleep-panic branch (`runHook(Sleep)` recovered) calls `endBusySlot` then `dispose`.
- `reclaim/table.go` `reclaimLocked` — Wake-panic branch calls `endBusySlot` then `dispose`.
- `reclaim/table_test.go` `TestTable_OpenBackgroundDoesNotPanic` — `Open(context.Background(), ...)` is accepted.
- `reclaim/table_test.go` `nilDoneCtx` / `TestTable_HolderWithoutDoneChannelIsPolled` — `Done()` is nil; cancel sets `Err()`; the holder MUST still be dropped.
- `reclaim/table_test.go` `TestTable_ResetLogsOrphanThenDisposeAndKeepsNextIncarnation` — a stale holder drop MUST NOT sleep a later incarnation of the same key.
- `openspec/specs/std_go_reclaim_context-lease/spec.md` — nil-`Done` holders stay live until `ctx.Err()` is set; "Every goroutine the table starts for a key SHALL exit once that key's holder contexts are Done and its incarnation has ended."; "A stale holder drop from a previous incarnation or from `Reset` MUST NOT change a later incarnation of the same key."
- Dest `reclaim/` has no `repro_nildone_watcher_test.go`. The validated reproducer lives only on the caller checkout (`reclaim/repro_nildone_watcher_test.go`, `TestRepro_NilDoneHolderLeaksWatchGoroutine`). Ticket recorded a fail on `c5118f1`; dest HEAD is `c230315`. `waitCtx` on dest still has only the `ctx.Err()` exit.

## Desired
1. Land the existing reproducer verbatim as `reclaim/repro_nildone_watcher_test.go` so `go test ./reclaim` FAILS. Do not rewrite or weaken it. Implement copies it; prepare does not.
2. Then give `watch` a second exit: stop polling once this incarnation is over. Do not call `drop` on that path.
3. Pick one of the two ticket shapes in explore (not prepare) and justify it in `devstate/explore.md`:
   1. Preferred: per-incarnation finished channel on `slot`, closed exactly once on every ending path (`endBusySlot`, `unmapAfterClose`, non-enforce `expire`, `Reset`); polling `select`s on it plus the tick.
   2. Cheaper: each tick, re-check under `t.mu` and return when state is `slotGone`.
4. Keep accepting `context.Background()` / nil-`Done` holders. Do not change `requireContext` to reject Background. Do not shorten the 20 ms tick.
5. Keep `TestTable_HolderWithoutDoneChannelIsPolled`: a nil-`Done` holder whose `Err()` is set MUST still be dropped.
6. Confirm the reproducer PASSES, `go test ./reclaim` stays green, and `go test ./... -short -count=1` (ignore pre-existing `apm_modules/.../evals/files [setup failed]`). This Windows host has no gcc; do not run `-race`.

## Affected
- `reclaim/table.go` (`watch` / `waitCtx` / `dropWhenDone`; if shape 1, `slot` plus ending paths)
- `reclaim/` tests (verbatim reproducer at implement; existing nil-Done and Reset-stale tests must stay green)
- `openspec/specs/std_go_reclaim_context-lease/spec.md` (goroutine-exit already stated; propose may add a scenario)
- `knowledge/devdocs/std_go_reclaim.md` (holder ctx / AfterFunc notes; watcher lifetime after incarnation end)

## Out of scope
- Rejecting `context.Background()` in `requireContext`
- Shortening the 20 ms poll interval
- Weakening or rewriting the reproducer
- Calling `drop` when the incarnation ends
- Ticket `2026-09-13-reclaim-bug-panic-ignores-enforce`: Sleep-panic in `drop` and Wake-panic in `reclaimLocked` honouring `Hooks.EnforceCloseBeforeOpen`
- Creating or editing `reclaim/BUGS.md`
- Reformatting or restructuring surrounding code
- Installing a C compiler or running `-race`
- Other packages (`simpleredis`, `tokenbucket`, `windowcounter`)

## Unknowns
- Which of the two watch-exit shapes explore will pick (ticket defers justification to `devstate/explore.md`).
- Whether shape 1's "every ending path" also needs an explicit close on `endMappedClose` (it already calls `unmapAfterClose`) and on the Sleep/Wake panic paths (they already call `endBusySlot`).
- Whether the reproducer's exact goroutine numbers at dest `c230315` match the ticket's `c5118f1` fail string. The missing exit is still in dest `waitCtx`.

## Tensions
- Spec already requires watcher exit when the incarnation has ended; dest `waitCtx` only exits on `ctx.Err()`. Ticket wins; dest is the bug.
- `watch` always `drop`s after `waitCtx`. Ticket forbids `drop` on the incarnation-end exit so a stale drop cannot change a later incarnation of the same key.
- Ticket line numbers (`dropWhenDone` ~332, `waitCtx` ~125, Sleep-panic ~375, Wake-panic ~300) are stale vs dest `c230315` (`dropWhenDone` after `finishBind`, `waitCtx` after `requireContext`, Sleep-panic inside `drop`, Wake-panic inside `reclaimLocked`). Same functions.
- Parallel ticket `2026-09-13-reclaim-bug-panic-ignores-enforce` edits the Sleep-panic and Wake-panic endings on the same file. This ticket must not change those branches; shape 1 still needs those paths to close a finished channel if they already funnel through `endBusySlot`.
