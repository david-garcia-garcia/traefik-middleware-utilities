# Explore
IssueKey: 2026-09-13-reclaim-bug-panic-ignores-enforce

## Concepts

```
  Sleep panic / Wake panic ending     dest today (flag set)
  --------------------------------    ----------------------
  runHook(Sleep|Wake) panics
    endBusySlot: unmap + close ready   key already absent
    dispose → Close still in flight   racing Open creates
         │                             second incarnation
         └── exclusive resource ─────► held twice (mmap, lock, port)

  agreed how (stored flag)
  ------------------------
  record createErr on the still-mapped slotBusy
  Close via dispose (outside t.mu)
  then endMappedClose / unmapAfterClose
  racing Open parks on ready
```

Shared helper between the two panic sites. Stored `EnforceCloseBeforeOpen` selects the order. Flag unset keeps `endBusySlot` then `dispose`. Site B still returns the wrapped Wake error and not the pointer; waiters replay `createErr`.

## Current (measured)

- Dest `origin/master` `c230315`. Site A `reclaim/table.go` `drop` Sleep panic: `endBusySlot(..., nil)` then `dispose`. Site B `reclaimLocked` Wake panic: `endBusySlot(..., err)` then `dispose`. Neither reads `storedHooks.EnforceCloseBeforeOpen`.
- Healthy endings already honour the flag: `drop` zero-grace after Sleep returns, and `expire`, call `endMappedClose` / `unmapAfterClose`. Those helpers do not record `createErr`. Waiters parked on `ready` replay `createErr` after it closes (`Open` `slotBusy`).
- Throwaway copy of the caller reproducer (only dest seam: `NewTable(graceNoRace)` → `New(Config{Grace: graceNoRace})`; dest has no `NewTable`) **failed** both tests at `-count=1`: `create of incarnation 2 ran while Close of 1 was blocked` (`TestRepro_SleepPanicUnmapsBeforeCloseWithEnforce` line 66, `TestRepro_WakePanicUnmapsBeforeCloseWithEnforce` line 140). File was not left in the tree.
- Ticket fail SHA `c5118f1` is behind dest; the two panic sites and the fail message still match on `c230315`.
- Spec Sleep-panic: "Close, unmap, and release waiters" with no order. Spec EnforceCloseBeforeOpen: unmapping before Close returns MUST NOT happen. Ticket wins; propose pins panic-path order so the two requirements stop reading as independent.
- Usage `knowledge/devdocs/std_go_reclaim.md` already says the ending path reads the stored flag; Sleep/Wake panic sentences still say Close then unmap with no flag. Do not rewrite it in explore (dest panic paths do not honour the flag yet).
- Identity: not in play (no client address / tenant / Host reconstruction).
- No third-party research: in-tree `reclaim` plus stdlib recover.

## Decisions

- Route both panic-recovery endings through one helper selected by the stored flag. Enforced: keep mapped `slotBusy` across Close, then existing `endMappedClose` / `unmapAfterClose`. Unset: keep `endBusySlot` then `dispose`.
- Record `createErr` on the slot before Close starts. Site A stays `nil` (waiters create). Site B stores the wrapped Wake error so waiters replay it after `ready` closes.
- Tests first: land the caller file in `reclaim/repro_enforce_panic_close_test.go`, dest constructor seam only. Do not weaken assertions. Then the helper.
- Do not fix the nil-Done watcher leak. Do not edit `reclaim/BUGS.md`. Stay out of `waitCtx`, `watch`, `dropWhenDone`.
- Sleep-panic ending stays grace-independent. Do not re-panic. Close still goes through `runHook` + `reclaim_hook_panic`.

## Open questions

- Q: What is the shared helper named, and does `endMappedClose` grow a `createErr` argument?
  Rank: additive asked — new helper this change creates; criterion 2 names one helper between the two sites; `endMappedClose` already has 2 callers (`drop` zero-grace, `expire`)
  Decision: assumed — `endBusyAfterPanic(key, incarnation, storedHooks, logger, createErr)` used only by the two panic sites. Flag unset: `endBusySlot` then `dispose`. Flag set: record `createErr` on the ended incarnation, occupy the key with a closer `slotBusy`, close the old `ready` so Wake waiters replay the error, `dispose`, then `unmapAfterClose` on the closer. Same-slot `endMappedClose` would give a later Open the Wake error (TestRepro_WakePanicUnmapsBeforeCloseWithEnforce).
  By: implement

- Q: How does the dest constructor seam get resolved without rewriting the reproducers?
  Rank: additive asked — criterion 1 names land the validated file; dest `New(Config)` replaced `NewTable`
  Decision: assumed — copy the file, then replace only `NewTable(graceNoRace)` with `New(Config{Grace: graceNoRace})` at both call sites. That is the dest API, not a weaken of the overlap assertions. Do not add a `NewTable` wrapper.
  By: explore
