## 1. Tests that fail on dest

- [x] 1.1 Add test-only `NewTable` in `reclaim/table_test.go` wrapping `New(Config{Grace: grace})`. Do not add it to `table.go`
- [x] 1.2 Copy `reclaim/repro_nildone_watcher_test.go` verbatim from the caller checkout (`D:/repositories/traefik-middleware-utilities/reclaim/repro_nildone_watcher_test.go`). Do not rewrite or weaken it
- [ ] 1.3 Run `go test ./reclaim -run TestRepro_ -count=1 -v` and confirm it FAILS (watchers still polling after Reset)

## 2. Watcher exit on incarnation end

- [ ] 2.1 Add `finished chan struct{}` on `slot`, created when the slot is first mapped
- [ ] 2.2 Add `closeFinished` (close then nil) and call it under `t.mu` from `endBusySlot`, `unmapAfterClose`, non-enforce `expire`, and `Reset` for awake/asleep. Skip busy/gone on Reset. Do not rewrite Sleep-panic or Wake-panic bodies
- [ ] 2.3 Change `watch`/`waitCtx` so the poll `select`s on `finished` plus the 20 ms tick. If `finished` wins, return without `drop`. If `ctx.Err()` is set while `finished` is still open, `drop` as today. Do not shorten the tick. Do not reject Background

## 3. Confirm and usage

- [ ] 3.1 Run `go test ./reclaim -run TestRepro_NilDoneHolderLeaksWatchGoroutine -count=1` and confirm it PASSES. Do not weaken the reproducer
- [ ] 3.2 Run `go test ./reclaim -count=1` and confirm existing tests stay green, including `TestTable_HolderWithoutDoneChannelIsPolled`, `TestTable_GoroutinesReturnToBaseline`, `TestTable_CancellableHoldersDoNotParkWaiters`, and Reset tests
- [ ] 3.3 Run `go test ./... -short -count=1` (ignore pre-existing `apm_modules/.../evals/files [setup failed]`)
- [ ] 3.4 Update `knowledge/devdocs/std_go_reclaim.md`: a nil-Done watcher exits when the incarnation ends; do not `drop` that holder
