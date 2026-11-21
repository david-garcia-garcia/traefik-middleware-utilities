# Explore
IssueKey: 2026-09-14-reclaim-bug-finished-race-lock-defer

## Concepts

```
  finishBind
       |
       v
  ctx.Err()  ---- slow first Err parks Open here ----
       |
       v
  dropWhenDone
       |
       +-- Done() != nil --> AfterFunc(drop)
       |
       +-- Done() == nil --> READ incarnation.finished   <--- NO t.mu
                              |
                              v
                           watch(finished)
                              |
                              v
                           waitCtx: select on finished
                              if finished == nil, that select never wakes
```

`slot.finished` is the per-incarnation channel `closeFinished` closes and nils under `t.mu`. `watch` is supposed to capture the channel at bind so a later nil does not hide the close. Capturing it *after* `closeFinished` already niled it hands `waitCtx` a nil channel: only `ctx.Err()` can stop the poll. A never-canceled nil-Done holder (`context.Background()`, Yaegi) then leaks the watcher, the key, and the dead slot.

`Reset` is the ending path that can `closeFinished` an *awake* incarnation while holders are still binding. Production last-holder drop increments/decrements `holders` under the same mutex, so it does not close `finished` while a bind is in the `finishBind` gap. The race is therefore a tests/CI path (`Reset` vs `Open`), but `-race` runs on every push and the leak is the same class PR #77 claimed fixed.

Mutex wedge: thirteen `t.mu.Lock()` sites, zero `defer t.mu.Unlock()`. A panic between lock and unlock skips unlock. Compiled Go dies; Yaegi recovers at the plugin boundary and every later `Open` on that table blocks. Reachable today: `Open` on zero-value `Table{}` assigns into a nil `items` map while holding `t.mu`.

Existing packets: `knowledge/devdocs/std_go_reclaim.md` (nil-Done watcher gotcha is the intended contract, currently false on the bind+Reset hole). Specs: `std_go_reclaim_context-lease` (holder watch, nil-Done exit, Close not under mutex) and `std_go_reclaim_value-lifecycle` (event order). Research: Yaegi AfterFunc and generics exist; panic-recovery at the plugin boundary is the owner's standing instruction, proxied in-repo by `recover()`.

## Decisions

- Defect 1 fix shape: read `finished` under `t.mu` via a tiny helper that `defer`s unlock; if the snapshot is nil, do not start `watch`. Lock alone is not the leak fix.
- Defect 2 fix shape: every lock-held region in `table.go` releases `t.mu` with `defer`. Extract helpers for `Open`, `drop`, and `reclaimLocked` because they interleave unlock across branches. Helpers return the channel the caller must `close()` outside the lock (`unmapAfterClose`, `endBusyAfterPanic`). Hooks and `slog` stay outside `t.mu`. `closeFinished` stays idempotent under the lock. One `close(ready)` per busy transition.
- `reclaimLocked` today is called *with* `t.mu` held and unlocks itself. Open's lock-held loop becomes a helper that returns a decision (`register`/`bind`/`reclaim`/`wait`/`retryGone`) plus the data the caller needs; Wake and `put` stay in the caller after unlock.
- Defect-2 repro: keep `TestRepro_PanicUnderTableMutexWedgesTable` but do not accept `t.Skip` when `Open` stops panicking. Add a white-box injection that pre-closes `slot.ready` so `close(incarnation.ready)` panics inside `put`'s locked region, then assert a later `Open`/`Reset` completes within a budget.
- Race repro: keep `slowErrNilDone` (slow first `ctx.Err()`). Do not widen with a slow `slog` handler.
- Field names `Table.mu`, `slot` fields, and `slotState` constants stay as they are (parallel ticket).
- Take the secondary `Open` error on a nil `items` map (`Table{}`), in addition to defer, matching nil-table / nil-logger. Production `Open` call site: `e2e/reclaimprobe/plugin.go` (uses `New`). Tests construct via `New` except this repro.
- Spec delta lives on `std_go_reclaim_context-lease` (synchronized finished snapshot, skip watch when nil, mutex released via defer, `Open` error on uninitialized table). `std_go_reclaim_value-lifecycle` stays event-order. Usage packet updated after apply if the gotcha still underspecifies the bind-path skip.

Reproduced on this worktree (uncommitted copies of the two repro files, not staged):

- `TestRepro_FinishedReadRaceLeaksWatcher`: FAIL `22 goroutines after every incarnation ended, baseline 2 and slack 2` (6.03s).
- `TestRepro_PanicUnderTableMutexWedgesTable`: FAIL `t.mu was still held after a panic under it (panic: assignment to entry in nil map)` (2.00s).
- Docker `golang:1.25` `-race` `TestRepro_FinishedReadRacesReset`: DATA RACE read `table.go:414` `dropWhenDone` vs write `table.go:192` `closeFinished`.

## Open questions

- Q: How should lock-held regions in `Open`, `drop`, and `reclaimLocked` be extracted so every unlock is `defer` without moving hook/`slog`/`close(ready)` under the mutex?
  Rank: additive asked — helpers are new units inside `table.go`; requirement Desired names helper extraction; exported `Open`/`Reset` callers keep working (1 production `Open` in `e2e/reclaimprobe/plugin.go`, rest tests)
  Decision: assumed — one helper per lock-held region that returns the decision and any channel the caller must close; `reclaimLocked`'s "called holding the lock" contract goes away by folding the asleep case into Open's lookup helper.
  By: explore

- Q: Which spec leaf owns the synchronized `finished` read, the nil-watch skip, deferred unlock, and `Open` error on `Table{}`?
  Rank: additive asked — requirement Desired names the behaviors; existing `std_go_reclaim_context-lease` already owns holder watch and "Close SHALL NOT run while the table mutex is held"
  Decision: assumed — modify `std_go_reclaim_context-lease`; do not add a third reclaim spec family; value-lifecycle stays event order.
  By: explore

- Q: Should `Open` return an error for zero-value `Table{}` (nil `items`) in addition to the defer refactor?
  Rank: additive asked — requirement Desired lists it as optional addition; enumerated callers already use `New`
  Decision: assumed — take it as an addition, never as a substitute for defer. Error copy matches the existing nil-table / nil-logger form.
  By: explore

- Q: Does this change need a Yaegi interp probe that recovers a panic under `t.mu`, or is compiled `recover()` enough?
  Rank: additive incidental — no acceptance line names a Yaegi probe; the compiled injection is the named repro
  Decision: assumed — compiled recover plus the `put` ready double-close injection is the proof; do not add a Traefik/Yaegi panic probe in this change.
  By: explore
