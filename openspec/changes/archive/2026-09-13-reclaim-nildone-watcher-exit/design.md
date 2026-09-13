## Context

See proposal.md — Why. Dest `reclaim/table.go` `dropWhenDone` starts `go t.watch` when `ctx.Done()` is nil. `watch` calls `waitCtx`, which returns only when `ctx.Err()` is set. `slotGone` is assigned in `endBusySlot`, `unmapAfterClose`, non-enforce `expire`, and `Reset` (awake/asleep). None of those stop `waitCtx`.

Explore: shape 1 (`finished` channel); test-only `NewTable`; do not edit Sleep-panic / Wake-panic bodies.

## Goals / Non-Goals

**Goals:**
- `watch` returns without `drop` when this incarnation has ended.
- `finished` is closed exactly once per incarnation.
- The verbatim reproducer fails on dest, then passes after the fix.
- Nil-Done whose `Err()` is set still drops.

**Non-Goals:**
- Reject `context.Background()` in `requireContext`.
- Shorten the 20 ms tick.
- Shape 2 (`t.mu` + `slotGone` each tick).
- Honour `EnforceCloseBeforeOpen` on Sleep-panic / Wake-panic (parallel ticket).
- Edit `reclaim/BUGS.md`.

## Decisions

### Per-incarnation `finished` channel (shape 1)
Create `finished chan struct{}` next to `ready` when the slot is first mapped. `waitCtx` (or `watch`) `select`s on it plus the 20 ms tick. If `finished` wins, return without `drop`. If `ctx.Err()` is set and `finished` is still open, `drop` as today.

Alternative considered: re-check `slotGone` under `t.mu` each tick. Rejected: it takes the mutex at 50 Hz per holder, including after the incarnation ended until the next tick. Ticket prefers shape 1.

### Close `finished` on the four `slotGone` writers, once
Helper `closeFinished` under `t.mu`: close then nil. Call sites: `endBusySlot`, `unmapAfterClose`, non-enforce `expire`, `Reset` for awake/asleep. `endMappedClose` is not a site (`unmapAfterClose`). Sleep-panic and Wake-panic already call `endBusySlot` — do not rewrite those branches. `Reset` skips busy/gone so the owner can close without a double-close panic.

Alternative considered: `sync.Once`. Rejected: every `slotGone` writer already holds `t.mu`; nil-after-close is enough.

### Tests first, verbatim reproducer
Copy `reclaim/repro_nildone_watcher_test.go` from the caller checkout with no edits. Add test-only `NewTable` in `table_test.go` (`New(Config{Grace: grace})`) so it compiles. Do not restore production `NewTable`. Confirm `go test ./reclaim -run TestRepro_` fails, then apply `table.go`, then the same test passes.

## Risks / Trade-offs

- [Risk] Double `close` on `finished` panics. → Mitigation: close-then-nil under `t.mu`; Reset does not signal busy/gone.
- [Risk] `watch` still `drop`s after incarnation end and decrements a later key. → Mitigation: incarnation-end exit returns before `drop`. `drop` still no-ops on `slotGone` for the old pointer; the spec forbids relying on that for a later incarnation.
- [Risk] Parallel ticket edits the same ending functions. → Mitigation: add only `closeFinished(...)` at those sites; do not reformat or rename.
- [Trade-off] `select` + ticker vs `time.Sleep` in `waitCtx`. Same 20 ms interval; ticker can wake immediately on `finished`.

## Migration Plan

Library bugfix. No API change. Rollback is revert. No stored data.
