## 1. Failing repros first

- [x] 1.1 Copy `reclaim/repro_finished_race_test.go` verbatim from `D:/repositories/traefik-middleware-utilities/reclaim/repro_finished_race_test.go`. Keep `slowErrNilDone` (slow first `ctx.Err()`). Do not widen the window with a slow slog handler
- [x] 1.2 Copy `reclaim/repro_mutex_wedge_test.go` from the same checkout, then extend it: add a white-box injection that pre-closes a slot's `ready` so `close(incarnation.ready)` panics inside `put`'s locked region, recover that panic, and assert a later `Open`/`Reset` completes within a budget. Do not treat `t.Skip` when zero-value `Open` no longer panics as done
- [x] 1.3 Run `go test -count=1 -timeout 10m -run TestRepro_FinishedReadRaceLeaksWatcher ./reclaim/` and `TestRepro_PanicUnderTableMutexWedgesTable` and confirm FAIL. Run Docker `golang:1.25 go test -race -count=1 -timeout 10m -run TestRepro_FinishedReadRacesReset ./reclaim/` and confirm DATA RACE at `table.go:414` vs `table.go:192`

## 2. Finished snapshot and nil skip

- [x] 2.1 Extract a tiny helper that reads `incarnation.finished` under `t.mu` with `defer Unlock`. In `dropWhenDone`, if that snapshot is nil, return without starting `watch`; otherwise `go t.watch` with the snapshot
- [x] 2.2 Confirm `TestRepro_FinishedReadRaceLeaksWatcher` PASSES. Confirm Docker `-race` `TestRepro_FinishedReadRacesReset` PASSES (no DATA RACE)

## 3. Defer-unlock every lock-held region

- [x] 3.1 Extract Open's lock-held loop body into a helper that `defer`s unlock and returns a decision (`register` / `bind` / `reclaim` / `wait` / `retryGone`) plus incarnation, value, ready, and stored hooks as needed. Fold `reclaimLocked`'s called-holding-the-lock contract into that lookup; Wake stays in the caller after unlock
- [x] 3.2 Extract drop's lock-held regions into helpers that `defer` unlock. Busy-wait returns the `ready` channel to wait on; last-holder snapshot (logger, hooks, grace) returns after unlock
- [x] 3.3 Convert every remaining lock-held region in `table.go` (`endBusySlot`, `unmapAfterClose`, `endBusyAfterPanic`, `put` publish, `expire`, `Reset`) to `defer Unlock`. Helpers that must close `ready` outside the lock return that channel; the caller closes it. Do not move hooks or `slog` under `t.mu`. Do not rename `Table.mu`, `slot` fields, or `slotState` constants. `closeFinished` stays idempotent under the lock
- [x] 3.4 `Open` returns an error on a nil `items` map (`Table{}`), same class as nil table / nil logger. This is an addition, not a substitute for 3.1–3.3

## 4. Prove and usage

- [x] 4.1 Confirm the lock-held panic injection PASSES (later `Open`/`Reset` completes). Confirm all three `TestRepro_*` in the two files PASS
- [x] 4.2 Run `go test -count=1 -timeout 10m ./reclaim/` green. Run Docker `golang:1.25 go test -race -count=1 -timeout 10m ./reclaim/` green
- [x] 4.3 Measure statement coverage: `go test -count=1 -covermode=atomic -coverprofile=<tmp>.cover -timeout 10m ./reclaim/` then `go tool cover -func=<tmp>.cover`. Must not be lower than DestBranch plus these repros (94.4% plus repro lines)
- [x] 4.4 Update `knowledge/devdocs/std_go_reclaim.md`: bind-path skip when `finished` is already nil; mutex released with `defer` so a recovered panic cannot wedge the table
