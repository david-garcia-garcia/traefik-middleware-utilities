## Why

A holder whose `Done()` is nil cannot use `context.AfterFunc`, so the table starts `go t.watch` and `waitCtx` polls `ctx.Err()` with no other exit. `context.Background()` never sets it, so after the incarnation has ended the watcher still polls and keeps the slot reachable for the life of the process.

## What Changes

- Give `watch` a second exit: stop polling once this incarnation is over, and return without calling `drop`.
- Park a per-incarnation `finished` channel on `slot`, closed exactly once on every `slotGone` path (`endBusySlot`, `unmapAfterClose`, non-enforce `expire`, `Reset` for awake/asleep).
- Land the existing reproducer `reclaim/repro_nildone_watcher_test.go` so `go test ./reclaim` fails on dest, then passes after the fix.
- Add a test-only `NewTable` wrapper in `reclaim/table_test.go` so that verbatim file compiles against dest `New(Config)`.
- Spec `std_go_reclaim_context-lease` gains a scenario: never-canceled nil-Done holder plus incarnation end (`Reset`) — watchers exit and MUST NOT `drop`.
- Usage packet `knowledge/devdocs/std_go_reclaim.md` notes that a nil-Done watcher exits when the incarnation ends.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_reclaim_context-lease`: every goroutine the table starts for a key still SHALL exit once holders are Done and the incarnation has ended. A nil-Done watcher whose incarnation has ended SHALL exit without `drop`. A nil-Done holder whose `Err()` is set SHALL still be dropped. `Open` still accepts `context.Background()`.

## Impact

- `reclaim/table.go` (`slot`, `watch` / `waitCtx`, `endBusySlot`, `unmapAfterClose`, `expire`, `Reset`; not Sleep-panic / Wake-panic bodies)
- `reclaim/table_test.go` (test-only `NewTable`)
- `reclaim/repro_nildone_watcher_test.go` (verbatim copy)
- `openspec/specs/std_go_reclaim_context-lease/spec.md`
- `knowledge/devdocs/std_go_reclaim.md`
- Parallel ticket `2026-09-13-reclaim-bug-panic-ignores-enforce` also edits `endBusySlot` / `unmapAfterClose` / `expire` — this change only adds a `closeFinished` call at those sites
