# Nil-Done holder watch goroutine leaks after incarnation ends

Bug: in `reclaim/table.go`, a holder context whose `Done()` returns nil cannot be watched with `context.AfterFunc`, so `dropWhenDone` (around `:332`) starts `go t.watch(...)`, and `watch` calls `waitCtx` (around `:125`), which polls:

```go
func waitCtx(ctx context.Context) {
	if done := ctx.Done(); done != nil {
		<-done
		return
	}
	for ctx.Err() == nil {
		time.Sleep(20 * time.Millisecond)
	}
}
```

The ONLY exit is `ctx.Err()` becoming non-nil. `context.Background()` never sets it, so that goroutine polls at 50 Hz for the life of the process. It keeps polling long after the incarnation it was watching has ENDED — `Table.Reset()` sleeps, closes and unmaps everything, and the watcher is still spinning with nothing left to drop — and it keeps `key` and the `*slot` (hence the stored value) reachable, so it leaks memory too.

This is reachable through the public API without doing anything the table rejects: `requireContext` only rejects a nil context, and `TestTable_OpenBackgroundDoesNotPanic` deliberately locks that `Open(context.Background(), ...)` is accepted.

Spec `openspec/specs/std_go_reclaim_context-lease/spec.md` requires: "Every goroutine the table starts for a key SHALL exit once that key's holder contexts are Done and its incarnation has ended." The incarnation ending is the half that is not honoured.

Agreed how (implement this, not a different design):
Give `watch` a SECOND exit: stop polling once this incarnation is over, because a holder cannot be dropped from a slot that is already gone.

Two acceptable shapes — pick one and justify it in `devstate/explore.md` (explore, not prepare):
1. Preferred: park a per-incarnation "finished" channel on the `slot`, closed on EVERY ending path (`endBusySlot`, `unmapAfterClose`, the non-enforce branch of `expire`, and `Reset`), and have the polling loop select on it alongside the tick. This does no work while idle. Be exhaustive about the ending paths and make sure the channel is closed exactly once per incarnation (a double `close` panics).
2. Cheaper: have `watch` re-check under `t.mu` on each tick and return when the slot's state is `slotGone`. No new slot field, but it costs a mutex acquisition per 20 ms per holder.

CRITICAL: the watcher must exit WITHOUT calling `drop`. Do NOT drop the holder when the incarnation ends — that would invert the ownership and could decrement a LATER incarnation's holder count for the same key, and the spec explicitly requires that "A stale holder drop from a previous incarnation or from `Reset` MUST NOT change a later incarnation of the same key."

Do NOT reject `context.Background()` in `requireContext` as the fix: the spec accepts nil-`Done` holders and the Yaegi holder shape is exactly that (see `nilDoneCtx` in `reclaim/table_test.go` and `TestTable_HolderWithoutDoneChannelIsPolled`). Do NOT shorten the 20 ms tick — the defect is the missing exit, not the interval. Do NOT break `TestTable_HolderWithoutDoneChannelIsPolled`, which requires that a nil-`Done` holder IS still dropped once its `Err()` is set.

Reproducer already written and validated — reuse it, do not rewrite. Source (read-only from caller checkout): `D:/repositories/traefik-middleware-utilities/reclaim/repro_nildone_watcher_test.go`. Copy that file verbatim into `reclaim/` in YOUR worktree as the failing product test during IMPLEMENT, not now. It is self-contained apart from helpers in `reclaim/table_test.go`. One test: `TestRepro_NilDoneHolderLeaksWatchGoroutine`. On origin/master at `c5118f1` it FAILS with `25 goroutines after every incarnation ended, baseline 5 and slack 2: 20 watchers are still polling`.

Prepare does NOT implement the fix and does NOT copy the reproducer into product code yet. Ground the requirement, qualify, stub PR only.

Order of later work (record in requirement Desired, do not execute):
1. Land the reproducer so `go test ./reclaim` FAILS.
2. Then implement the agreed how in `reclaim/table.go`.
3. Confirm reproducer PASSES and `./reclaim` stays green; also `go test ./... -short -count=1` (ignore pre-existing `apm_modules/.../evals/files [setup failed]`).

Do NOT weaken the reproducer. Do NOT reject Background. Do NOT shorten the tick.

## Parallel-ticket boundary
A second ticket `2026-09-13-reclaim-bug-panic-ignores-enforce` is running in parallel on the same file. It is changing the Sleep-panic branch of `drop` (around `:375`) and the Wake-panic branch of `reclaimLocked` (around `:300`) so those endings honour `Hooks.EnforceCloseBeforeOpen`. Do NOT fix or touch that bug. Do NOT create or edit `reclaim/BUGS.md`. Do not reformat or restructure surrounding code. Note the collision on the delivery card so the caller knows where to expect it.

`-race` cannot run on this Windows host (no gcc). Do not try to install a compiler.
