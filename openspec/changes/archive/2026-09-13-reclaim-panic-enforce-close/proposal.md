## Why

`Hooks.EnforceCloseBeforeOpen` is supposed to keep a key mapped until Close returns, so a later Open cannot create a second incarnation that holds the same exclusive resource. Dest already does that on the healthy zero-grace drop and expire paths. The two panic-recovery endings (Sleep in `drop`, Wake in `reclaimLocked`) still unmap first, then Close, so a racing Open creates while Close is in flight.

## What Changes

- Route both panic-recovery endings through one helper selected by the stored `EnforceCloseBeforeOpen` (never a later Open's argument).
- When the stored flag is set: keep the slot mapped `slotBusy` across Close, then unmap with the existing `endMappedClose` / `unmapAfterClose`. Do not close `ready` before Close. Record `createErr` on the slot before Close starts so waiters still replay it after `ready` closes.
- When the stored flag is unset: keep dest `endBusySlot` then `dispose`. Default overlap stays.
- Site B still returns `fmt.Errorf("reclaim: wake %q: panic: %v", key, recovered)` and not the stored pointer.
- Land the two TestRepro_* product tests first so `go test ./reclaim` fails on dest, then the helper.
- Pin Sleep-panic and Wake-panic Close-versus-unmap order in the live specs so they no longer read as independent of EnforceCloseBeforeOpen.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_reclaim_value-lifecycle`: Sleep-panic and Wake-panic endings honour stored `EnforceCloseBeforeOpen` for unmap-versus-Close order. Flag unset keeps unmap-then-Close. Wake panic still publishes `createErr` before Close so waiters receive that error.
- `std_go_reclaim_context-lease`: Sleep-panic and Wake-panic are ending paths that keep the key stored until Close returns when the stored flag is set. Sleep-panic is grace-independent.

## Impact

- `reclaim/table.go` (`drop` Sleep panic, `reclaimLocked` Wake panic, shared helper)
- `reclaim/repro_enforce_panic_close_test.go` (land; dest constructor `New(Config{Grace: graceNoRace})`)
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md`
- `openspec/specs/std_go_reclaim_context-lease/spec.md`
- `knowledge/devdocs/std_go_reclaim.md`
- No `Open` signature change. No change to default overlap. No `Reset` reorder. No `waitCtx` / `watch` / `dropWhenDone`.
