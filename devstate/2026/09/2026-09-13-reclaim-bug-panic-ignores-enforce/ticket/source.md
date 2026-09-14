# Panic-recovery endings ignore EnforceCloseBeforeOpen and unmap before Close

In `reclaim/table.go`, `Hooks.EnforceCloseBeforeOpen` is supposed to keep a key mapped `slotBusy` until the `Close` hook returns, so a concurrent `Open` of that key waits instead of creating a second incarnation. It exists for values that own something that cannot be held twice: an mmap, a file lock, a listening port, a connection.

That stored flag is read on exactly TWO paths today: `drop`'s zero-grace branch and `expire`. The two panic-recovery ending paths do NOT consult it. Both call `endBusySlot` (which unmaps the key AND closes `ready`) and only THEN call `dispose` (which runs `Close`). So while `Close` is still in flight the key is already absent, and a racing `Open` creates a second incarnation that holds the same exclusive resource.

Site A, the Sleep-panic branch of `drop`, currently around `reclaim/table.go:375-382`:
```go
if recovered := runHook(storedHooks.Sleep); recovered != nil {
	logger.Error(MsgHookPanic, "key", key, "hook", "sleep", "panic", recovered)
	t.endBusySlot(key, incarnation, nil)
	dispose(key, storedHooks, logger)
	return
}
```

Site B, the Wake-panic branch of `reclaimLocked`, currently around `reclaim/table.go:300-307`:
```go
if recovered := runHook(storedHooks.Wake); recovered != nil {
	err := fmt.Errorf("reclaim: wake %q: panic: %v", key, recovered)
	t.endBusySlot(key, incarnation, err)
	dispose(key, storedHooks, logger)
	return nil, err
}
```

Agreed how (implement this, not a different design):
Route BOTH panic-recovery endings through the same Close-before-unmap machinery the healthy paths already use, selected by the STORED flag (never a later Open's argument). Share one helper between the two sites rather than writing the logic twice.
- When the stored `EnforceCloseBeforeOpen` is set: keep the slot mapped `slotBusy` across `Close`, then end with the existing `endMappedClose` / `unmapAfterClose` (`reclaim/table.go:185` and `:172`), so a racing `Open` parks on `ready` and creates only after `Close` returned or its panic was recovered.
- When it is NOT set: keep today's `endBusySlot`-then-`dispose` order. That overlap is the documented default, and `TestTable_ZeroGraceCreateDoesNotWaitForClose` locks it. Do not change default behaviour.
- On site B the reclaiming `Open` must still return `fmt.Errorf("reclaim: wake %q: panic: %v", key, recovered)` and must still NOT return the stored pointer. Only the unmap-versus-`Close` ORDER changes.
- Concurrent waiters parked on `ready` must still receive that same Wake error by replaying `createErr` (spec `std_go_reclaim_value-lifecycle`: "Concurrent waiters on that transition SHALL receive the same error"). That means `createErr` must be recorded on the slot BEFORE `Close` starts, even though `ready` is now closed AFTER `Close` returns. Do not lose that.
- Do NOT close `ready` before `Close` on the enforced path. Do NOT run `Close` while holding `t.mu`. Do NOT make this conditional on grace: the Sleep-panic ending is grace-independent. Do NOT re-panic. Do NOT let a `Close` panic escape (it can run on an AfterFunc goroutine and would kill the process); `runHook` + the `reclaim_hook_panic` error line must still work.

Reproducers already written and validated — reuse them, do not rewrite. Source file (caller tree, copy later at implement, not now): `D:/repositories/traefik-middleware-utilities/reclaim/repro_enforce_panic_close_test.go`. Two tests: `TestRepro_SleepPanicUnmapsBeforeCloseWithEnforce` and `TestRepro_WakePanicUnmapsBeforeCloseWithEnforce`. Both FAIL on origin/master at c5118f1 with `create of incarnation 2 ran while Close of 1 was blocked`.

Order of work for later implement (record as Desired):
1. Land the reproducer file so `go test ./reclaim` FAILS on the branch.
2. Then implement the agreed how in `reclaim/table.go`.
3. Confirm both TestRepro_* PASS and `go test ./reclaim -count=1` plus `go test ./... -short -count=1` (ignore pre-existing `apm_modules/.../evals/files [setup failed]`).

Do NOT weaken reproducers. Do NOT change default (flag-unset) overlap. Do NOT fix the nil-Done watcher leak. Do NOT edit reclaim/BUGS.md.

OpenSpec leaves: `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` and `openspec/specs/std_go_reclaim_context-lease/spec.md`. Today they conflict: Sleep-panic says "Close, unmap, and release waiters" without ordering those three, while EnforceCloseBeforeOpen says unmapping before Close returns MUST NOT happen for that incarnation. Usage doc: `knowledge/devdocs/std_go_reclaim.md`.
