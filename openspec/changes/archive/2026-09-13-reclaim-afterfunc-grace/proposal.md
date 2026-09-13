## Why

Interpreted `reclaim` grace wait parks in Yaegi v0.16.1 `interp._select`. On Go 1.21.13, concurrent expire of 16 keys missed Close and left `interp._select.func4` at `run.go:3815` spawned from `go callf(in)` (`run.go:1322`). `expire`, `dispose`, and `endMappedClose` never run. A later `Open` can still reclaim the leaked incarnation. `e2e/reclaimprobe/plugin.go` arms `DefaultGrace` on every Traefik drop. PR #75 already replaced the same shape in `windowcounter` with `time.AfterFunc`.

## What Changes

- Replace `go waitGraceOrWake` + `select` on `timer.C` and `woken` with `time.AfterFunc(grace, expire)`. Store the timer on the slot. `reclaimLocked` and `Reset` `Stop()` it instead of `close(woken)`.
- Keep the contract: grace wait is not on the drop caller; wake cancels expiry; `expire` guards stay (`slotAsleep`, `holders > 0`, `EnforceCloseBeforeOpen` → `slotBusy`).
- Yaegi constraint note on `reclaim/table.go` matching `windowcounter/limiter.go` / `simpleredis/resp.go`.
- Interpreted regression: `TestYaegi_GraceExpireDoesNotHang` (3s watchdog), the dest concurrent-expire hang.
- Do not convert `go t.watch` (`waitCtx` is `<-done` or an `Err` poll; 80 interpreted nil-Done holders Closed on Go 1.21.13).
- Target `master`. Do not stack on reclaim bug branches. #77 and #78 still have `waitGraceOrWake`.

## Capabilities

### New Capabilities

- None. Fold into the existing value-lifecycle leaf.

### Modified Capabilities

- `std_go_reclaim_value-lifecycle`: positive-grace expire is a stdlib `AfterFunc` timer, not an interpreted `go`+`select` goroutine. Sleep/wake/close order is unchanged.

## Impact

- `reclaim/table.go` (`waitGraceOrWake` removed; `woken` replaced by `graceTimer`)
- `reclaim/yaegi_test.go` (watchdog)
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` (after archive)
- `knowledge/devdocs/std_go_reclaim.md` Yaegi gotcha
- No `watch` conversion. No grace-duration change. No #77/#78 rebase.
