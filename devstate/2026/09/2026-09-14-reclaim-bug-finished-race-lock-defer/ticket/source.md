# Fix unsynchronized `slot.finished` read (watcher leak) and panic-under-mutex wedge in `reclaim`

Two defects in `reclaim/table.go` on DestBranch `master`:

## Defect 1: unsynchronized read of `slot.finished` reopens the watcher leak

`dropWhenDone` reads `incarnation.finished` WITHOUT holding `t.mu` (`reclaim/table.go:414`), while `closeFinished` closes that field and sets it nil UNDER `t.mu` (`reclaim/table.go:192`). It is the only `slot` field accessed outside the mutex in the file.

Reachable when a nil-`Done` holder binds while `Reset` ends the incarnation. When `Reset` nils `finished` before that read, the watcher is started with a NIL channel, so `waitCtx` selects on nil forever and only cancellation can stop it. A holder that is never canceled (`context.Background()`, the Yaegi holder shape) then polls `ctx.Err` every 20ms for the life of the process and keeps the key, the dead slot and its value reachable. That is exactly the leak `reclaim/repro_nildone_watcher_test.go` certifies as fixed — PR #77's fix has a hole on the bind path.

Measured evidence:
- `TestRepro_FinishedReadRacesReset` under Docker `-race`: `WARNING: DATA RACE`, read at `table.go:414` (`dropWhenDone` <- `finishBind` <- `Open`) against previous write at `table.go:192` (`closeFinished` <- `Reset`).
- `TestRepro_FinishedReadRaceLeaksWatcher` without `-race`: fails with `22 goroutines after every incarnation ended, baseline 2 and slack 2`, i.e. exactly one leaked watcher per key over 20 keys.

Fix the caller already applied and verified green:

```go
func (t *Table) dropWhenDone(ctx context.Context, key string, incarnation *slot) {
	if ctx.Done() != nil {
		context.AfterFunc(ctx, func() { t.drop(key, incarnation) })
		return
	}
	t.mu.Lock()
	finished := incarnation.finished
	t.mu.Unlock()
	if finished == nil {
		// This incarnation ended between the bind and here, so there is no holder left to drop
		// and no channel left to wake a watcher. Starting one would poll ctx.Err forever.
		return
	}
	go t.watch(ctx, key, incarnation, finished)
}
```

Reading under the lock alone is NOT sufficient — it removes the race but still hands `watch` a nil channel and keeps the leak. The nil check is the part that fixes the leak. Note this interacts with defect 2: express the locked read with `defer` (extract a tiny helper) rather than the bare Lock/Unlock shown above.

TRAP if you rework the race repro: do NOT widen the window with a slow `slog` handler. The working mechanism is a holder whose first `ctx.Err()` call is slow: `finishBind` calls `Err()` immediately before the read, so that parks the binding `Open` in the exact gap without taking any lock the ending goroutine also takes. Keep that mechanism.

## Defect 2: a panic raised under `t.mu` wedges the table for the life of the process

No lock-held region in `reclaim/table.go` releases `t.mu` with `defer`: 13 `t.mu.Lock()` calls and 20 `t.mu.Unlock()` calls, zero deferred. Any panic between a lock and its unlock skips that unlock.

Why it matters: in a normal Go binary an unrecovered panic ends the process. Under Yaegi — which is how Traefik runs this package — the panic is recovered at the plugin boundary and the process keeps running with `t.mu` held, so every later `Open` on every key of that table blocks forever. Standing instruction: always release mutexes with `defer`.

Reachable today through the exported API: `Open` on a zero-value `Table{}` takes `t.mu` and then assigns into the nil `items` map, which panics `assignment to entry in nil map` with the lock held. `TestRepro_PanicUnderTableMutexWedgesTable` recovers the panic the way the plugin boundary would and then calls `Reset`, which never acquires the mutex.

Requirements:
1. Convert EVERY lock-held region in `table.go` to release `t.mu` via `defer`. A nil-map guard alone is NOT the fix. The supplied repro contains `t.Skip` when `Open` no longer panics, so merely adding a guard would turn it green while leaving the whole defect class in place.
2. Therefore replace or extend that repro with an injection that panics INSIDE a lock-held region and asserts the table is still usable afterwards. Suggested white-box injection: pre-close a slot's `ready` channel so that `close(incarnation.ready)` panics inside `put`'s locked region. Assert a later `Open`/`Reset` completes within a budget instead of hanging.
3. `Open`, `drop` and `reclaimLocked` interleave lock and unlock across branches, so `defer` means extracting each locked region into a small helper that returns the decision/data the caller needs. Note `reclaimLocked` is currently called WITH `t.mu` held and releases it; that contract has to change or be encapsulated.
4. Preserve: `unmapAfterClose` and `endBusyAfterPanic` `close()` channels OUTSIDE the lock on purpose. Keep that — have the helper return the channel and close it in the caller.
5. Invariants: hooks and every `slog` call run outside `t.mu`; `close(slot.ready)` happens exactly once per busy transition on every ending path; `closeFinished` stays idempotent under the lock. Do not introduce a double-close panic.
6. Secondary, as an ADDITION and not as the fix: `Open` rejects a nil table and a nil logger with an error but panics on `Table{}`. Consider returning an error there too.

## Coordination with a parallel ticket

A second ticket `2026-09-14-reclaim-ending-path-test-coverage` adds ONLY `reclaim/table_gaps_test.go` and does not touch `table.go`. Keep the `Table.mu` field name and the `slot` field and `slotState` constant names stable.

## Acceptance

- `go test -count=1 -timeout 10m ./reclaim/` green
- Docker `-race`: `docker run --rm -v "<worktree>:/src" -w /src golang:1.25 go test -race -count=1 -timeout 10m ./reclaim/`
- All three supplied repro tests present and passing (defect-2 one reworked per requirement 2)
- Statement coverage for `reclaim` not lower than on `master` (94.4% plus the repro lines)
- Stub PR opened during prepare on GitHub, CI measured green, delivery card on the PR summary

## Repro tests (exist untracked in MAIN checkout; do not commit from main)

- `D:/repositories/traefik-middleware-utilities/reclaim/repro_finished_race_test.go`
- `D:/repositories/traefik-middleware-utilities/reclaim/repro_mutex_wedge_test.go`

Read-only reference (do not commit/edit from main):
- `D:/repositories/traefik-middleware-utilities/knowledge/debt/2026-09-14-reclaim-race-leak-audit.md`
- `D:/repositories/traefik-middleware-utilities/reclaim/BUGS.md` (bug 3 + note that `-race` was never part of that hunt)
