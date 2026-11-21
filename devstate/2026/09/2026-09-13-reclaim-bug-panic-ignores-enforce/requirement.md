# Requirement
IssueKey: 2026-09-13-reclaim-bug-panic-ignores-enforce

## Problem
Panic-recovery endings in `reclaim/table.go` ignore stored `Hooks.EnforceCloseBeforeOpen`. Both the Sleep-panic branch of `drop` and the Wake-panic branch of `reclaimLocked` call `endBusySlot` (unmap + close `ready`) and only then `dispose` (`Close`). While `Close` is still in flight the key is already absent, so a racing `Open` creates a second incarnation that can hold the same exclusive resource (mmap, file lock, listening port, connection) the flag exists to serialize.

## Current (code)
- `Hooks.EnforceCloseBeforeOpen` is stored at put and documented as keeping the key mapped `slotBusy` until `Close` returns. `reclaim/table.go` `Hooks`.
- Healthy endings consult the stored flag: `drop` zero-grace (`reclaim/table.go` after Sleep returns) and `expire` call `endMappedClose` / `unmapAfterClose` when the flag is set. `reclaim/table.go` `endMappedClose`, `unmapAfterClose`, `drop`, `expire`.
- Site A — `drop` Sleep panic: logs `reclaim_hook_panic`, `endBusySlot(key, incarnation, nil)`, then `dispose`. Does not read `storedHooks.EnforceCloseBeforeOpen`. Grace-independent. `reclaim/table.go` `drop`.
- Site B — `reclaimLocked` Wake panic: wraps `fmt.Errorf("reclaim: wake %q: panic: %v", key, recovered)`, `endBusySlot(key, incarnation, err)`, then `dispose`, returns `(nil, err)`. Does not read the stored flag. `reclaim/table.go` `reclaimLocked`.
- `endBusySlot` sets `createErr`, `slotGone`, unmaps, and closes `ready` before `Close`. `reclaim/table.go` `endBusySlot`.
- `endMappedClose` runs `dispose` while still mapped, then `unmapAfterClose`. It does not record `createErr`. `unmapAfterClose` unmaps and closes `ready` after `Close`. `reclaim/table.go` `endMappedClose`, `unmapAfterClose`.
- Flag unset / default overlap is locked by `reclaim/table_test.go` `TestTable_ZeroGraceCreateDoesNotWaitForClose`.
- Dest `reclaim/` has no `repro_enforce_panic_close_test.go`. Callers validated `TestRepro_SleepPanicUnmapsBeforeCloseWithEnforce` and `TestRepro_WakePanicUnmapsBeforeCloseWithEnforce` (copy at implement, not now). Dest HEAD is `c230315`; ticket named fail SHA `c5118f1`.
- Spec Sleep-panic: end with "Close, unmap, and release waiters" with no order among those three. Wake panic: "Close, unmap"; concurrent waiters SHALL receive the same error. `openspec/specs/std_go_reclaim_value-lifecycle/spec.md`.
- Spec EnforceCloseBeforeOpen: unmapping before `close` returns MUST NOT happen for that incarnation; when true, `ready` stays open across Close. `openspec/specs/std_go_reclaim_value-lifecycle/spec.md`. Same keep-mapped-until-Close in `openspec/specs/std_go_reclaim_context-lease/spec.md`.
- Usage: flag defaults off; when set, later `Open` waits until Close returns. Sleep/Wake panic text still says Close then unmap with no flag. `knowledge/devdocs/std_go_reclaim.md`.

## Desired
1. Land `reclaim/repro_enforce_panic_close_test.go` so `go test ./reclaim` FAILS on the branch. Reuse the validated file; do not rewrite or weaken `TestRepro_SleepPanicUnmapsBeforeCloseWithEnforce` or `TestRepro_WakePanicUnmapsBeforeCloseWithEnforce`.
2. Then implement the agreed how in `reclaim/table.go`. Share one helper between site A and site B. Select by the stored flag, never a later `Open`'s argument.
3. Stored `EnforceCloseBeforeOpen` set: keep the slot mapped `slotBusy` across `Close`, then end with existing `endMappedClose` / `unmapAfterClose`, so a racing `Open` parks on `ready` and creates only after `Close` returned or its panic was recovered. Do not close `ready` before `Close` on this path. Do not run `Close` while holding `t.mu`.
4. Stored flag unset: keep today's `endBusySlot`-then-`dispose` order. Do not change default overlap.
5. Site B: still return `fmt.Errorf("reclaim: wake %q: panic: %v", key, recovered)` and not the stored pointer. Record `createErr` on the slot BEFORE `Close` starts so waiters replay that same Wake error after `ready` closes. Only unmap-versus-`Close` order changes.
6. Sleep-panic ending is grace-independent. Do not re-panic. `Close` panic must still be recovered via `runHook` and `reclaim_hook_panic`.
7. Confirm both TestRepro_* PASS and `go test ./reclaim -count=1` plus `go test ./... -short -count=1` (ignore pre-existing `apm_modules/.../evals/files [setup failed]`).

## Affected
- `reclaim/table.go` (Sleep-panic `drop`, Wake-panic `reclaimLocked`, shared helper around `endMappedClose` / `unmapAfterClose` / `createErr`)
- `reclaim/repro_enforce_panic_close_test.go` (land at implement)
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` (Sleep/Wake panic order vs EnforceCloseBeforeOpen)
- `openspec/specs/std_go_reclaim_context-lease/spec.md` (same)
- `knowledge/devdocs/std_go_reclaim.md` (Sleep/Wake panic vs flag)

## Out of scope
- Default (flag-unset) unmap-then-Close overlap
- Nil-Done watcher leak (`2026-09-13-reclaim-bug-nildone-watcher-leak`)
- Editing `reclaim/BUGS.md`
- Copying the reproducer during prepare
- `waitCtx`, `watch`, `dropWhenDone`
- Weakening the reproducers
- Other packages

## Unknowns
- Helper name and whether `endMappedClose` grows a `createErr` argument versus a new wrapper that records `createErr` then calls today's `endMappedClose`.
- Dest constructor is `New(Config)`; the validated reproducer calls `NewTable(graceNoRace)`. Copy-as-is may not compile until that seam is resolved without weakening the tests.
- Ticket fail SHA `c5118f1` vs current `origin/master` `c230315` — panic sites still match; confirm TestRepro_* still fail on this dest at implement.

## Tensions
- Spec Sleep-panic: "Close, unmap, and release waiters" with no order. Spec EnforceCloseBeforeOpen: unmapping before Close returns MUST NOT happen when the stored flag is set. Ticket: enforced panic endings keep mapped across Close; flag-unset keeps unmap-then-Close. Ticket wins; specs/usage catch up in propose.
- Spec Wake panic: concurrent waiters SHALL receive the same error. Enforced path closes `ready` after Close, so `createErr` must be stored before Close starts. Today's `endMappedClose` does not record `createErr`.
- Ticket line numbers (`drop` ~375, `reclaimLocked` ~300, `endMappedClose` ~185) are stale versus dest `c230315` (sites ~383 and ~308; helpers ~178/`191`). Same two branches.
- Reuse-do-not-rewrite vs dest `New(Config)` vs reproducer `NewTable`.
