## Context

See proposal.md for why. Dest `reclaim/table.go` `drop` Sleep panic and `reclaimLocked` Wake panic call `endBusySlot` (unmap + close `ready`) then `dispose`. Healthy zero-grace drop and `expire` already honour stored `EnforceCloseBeforeOpen` via `endMappedClose` / `unmapAfterClose`. Those helpers do not record `createErr`. Waiters parked on `ready` replay `createErr` after it closes.

Explore decisions: helper `endBusyAfterPanic`; dest constructor seam `New(Config{Grace: graceNoRace})`; do not grow `endMappedClose`.

## Goals / Non-Goals

**Goals:**
- Both panic sites share one helper selected by the stored flag.
- Enforced path: mapped across Close; `createErr` recorded before Close; `ready` closed after.
- Unset path: dest `endBusySlot` then `dispose`.
- Tests fail on dest, then pass after the fix.

**Non-Goals:**
- Change default overlap.
- Grow `endMappedClose`'s signature.
- Fix the nil-Done watcher leak. Edit `reclaim/BUGS.md`. Touch `waitCtx`, `watch`, `dropWhenDone`.
- Re-panic. Run Close under `t.mu`. Close `ready` before Close on the enforced path.

## Decisions

### Shared `endBusyAfterPanic`
`endBusyAfterPanic(key, incarnation, storedHooks, logger, createErr)` used only by the two panic sites. When the stored flag is set, record `createErr` under `t.mu` while still mapped `slotBusy`, then `endMappedClose`. When unset, `endBusySlot` then `dispose`.

Alternative considered: grow `endMappedClose` with a `createErr` argument. Rejected: the two healthy callers have no error to publish.

Alternative considered: duplicate the branch at both panic sites. Rejected: ticket requires one helper.

### Record `createErr` before Close
Site A passes `nil` so waiters create. Site B stores the wrapped Wake error so waiters replay it after `ready` closes. The reclaiming Open still returns that same error and not the pointer.

Alternative considered: close `ready` before Close so waiters wake immediately with `createErr`. Rejected: that unmaps (or at least releases waiters) before Close on the enforced path.

### Tests first, dest constructor seam only
Copy `reclaim/repro_enforce_panic_close_test.go` from the caller tree, replace only `NewTable(graceNoRace)` with `New(Config{Grace: graceNoRace})`. Do not weaken assertions. Confirm FAIL, then the helper, then PASS.

## Risks / Trade-offs

- [Risk] Recording `createErr` while still mapped lets a waiter that hasn't parked yet see it after `ready` closes. → Mitigation: that is the required waiter contract; Open `slotBusy` already replays `createErr`.
- [Risk] Sleep-panic `createErr` nil plus enforced mapping: waiters must stay parked until Close, then create. → Mitigation: do not close `ready` until `unmapAfterClose`.
- [Trade-off] Enforced Close still blocks a racing Open. Required: that is what the flag is for.

## Migration Plan

Library change. Callers that set `EnforceCloseBeforeOpen` get the documented Close-before-create contract on panic endings as well as healthy ones. Default overlap unchanged. Rollback is revert. No stored data.
