## Why

Interpreted buffered `windowcounter` `Sleep`/`Close` can hang until the 5-minute test timeout. Yaegi v0.16.1 `interp._select` misses `close(stop)` on the flush method goroutine, so `wg.Wait` never returns. Measured on Go 1.21.13: `WaitGroup.Wait` plus two `interp._select` frames. CI run 34766385613 killed the whole `windowcounter` binary at 300.008s. Exact-mode subtests do not start a flush worker and were not hung. `simpleredis` already left this pattern for `context.AfterFunc`.

## What Changes

- Replace `go flushLoop` + `select` on ticker/stop with a self-re-arming `time.AfterFunc` (compiled stdlib timer). Keep periodic flush.
- Keep Sleep flushes then stops; Wake restarts only when not closed, `sync_rate > 0`, not already running, and not stopping; Close is idempotent, flushes, refuses to restart; `stopping` still wins against concurrent Wake; `lastFlushErr` / `flushFailedAt` / `lastRedisOK` unchanged.
- Yaegi constraint note on `windowcounter/limiter.go` matching `simpleredis/resp.go`.
- Interpreted regression: `TestYaegi_BufferedShareSleepDoesNotHang` (3s watchdog) and `TestYaegi_MethodFlushStopDoesNotHang`.
- Supercede PR 61 for this deadlock (dest already has `stopping`). Do not merge that branch. Do not touch PR 69.
- Sibling sweep: note `reclaim` `waitGraceOrWake`; do not convert it here.

## Capabilities

### New Capabilities

- None. Fold into the existing Sleep/Wake/Close flush leaf.

### Modified Capabilities

- `std_go_windowcounter_sync-flush`: buffered flush is a stdlib `AfterFunc` timer, not an interpreted `go`+`select` goroutine. Sleep/Wake/Close contract is unchanged.

## Impact

- `windowcounter/limiter.go` (timer, Yaegi note)
- `windowcounter/limiter_test.go`, `windowcounter/yaegi_flush_stop_test.go`, `windowcounter/repro_sleep_wake_race_test.go` comments
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` (after archive)
- `knowledge/devdocs/std_go_windowcounter.md` Yaegi gotcha
- `knowledge/debt/` note for reclaim `waitGraceOrWake`
- No opportunistic idle-only flush. No tokenbucket. No PR 61 merge. No PR 69.
