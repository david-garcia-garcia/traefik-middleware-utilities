## Context

DestBranch `flushLoop` is `go` plus `select` on `ticker.C` and `stop`. Traefik and `TestYaegi_*` interpret that code. `simpleredis/resp.go` already documented Yaegi v0.16.1 `interp._select` races. Specs: `std_go_windowcounter_sync-flush`. Explore: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Interpreted Sleep/Close returns (no WaitGroup hang on a missed select).
- Periodic buffered flush remains.
- Sleep/Wake/Close and stopping-guard intent stay.
- Short-watchdog interpreted regression so a hang fails in seconds, not 300s.

**Non-Goals:**
- Opportunistic Take/Peek-only flush (idle deltas would sit unflushed).
- Converting reclaim `waitGraceOrWake` in this change.
- Merging PR 61 or touching PR 69.

## Decisions

1. **Self-re-arming `time.AfterFunc`.** Yaegi v0.16.1 exports `time.AfterFunc` in `stdlib/go1_21_time.go`. The timer goroutine is compiled. Alternative: opportunistic flush on Take/Peek — rejected; changes idle flush semantics. Alternative: keep `go`+`select` and hope — rejected; dump confirmed Wait + `_select`.

2. **`flushBusy WaitGroup` counts one outstanding timer/callback, not an interpreted select loop.** `Stop()` true → `Done` in `stopFlushAndWait`. `Stop()` false → `flushAfterTick` `Done`s when it sees `flushTimer==nil` or `stopping`. Alternative: no wait for in-flight tick — rejected; dest waited for `flushLoop` to exit.

3. **Keep `stopping`.** Wake is a no-op while Sleep/Close is detaching the timer. Dest already has this. Supercede PR 61's limiter rewrite; do not build on that branch.

4. **Permanent tests are interpreted `BufferedShare` with a 3s watchdog plus an isolated method-select probe.** The isolated probe did not hang in explore (1000 cycles); `BufferedShare` on Go 1.21.13 did. Both stay.

## Risks / Trade-offs

- [AfterFunc callback vs Stop race] → Mitigation: nil `flushTimer` and set `stopping` under `l.mu` before `Stop`; callback re-arms only when those are clear; `flushBusy` waits the in-flight tick.
- [Isolated probe may pass on unfixed dest] → Mitigation: `BufferedShare` watchdog is the dest hang; probe documents the forbidden shape.
- [reclaim still has `go`+`select`] → Mitigation: debt note, not this PR.

## Migration Plan

No signature change. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
