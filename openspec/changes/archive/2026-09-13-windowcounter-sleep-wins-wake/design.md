## Context

DestBranch `windowcounter.Limiter` already has `Sleep` / `Wake` / `Close`, `stop`, `ticker`, `wg`, `closed`. `takeFlushTickerLocked` sets `stop=nil` before `stopFlushAndWait` unlocks and `wg.Wait`. `Wake` starts a loop whenever `stop==nil` and not closed. Specs: `std_go_windowcounter_sync-flush`. Explore: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Sleep returns when Wake races it.
- Tests-first: failing `TestRepro_SleepWakeRaceHangs` on dest, then the flag, then that test returns.
- Wake after Sleep still starts. Wake after Close still does not.

**Non-Goals:**
- Changing Close to close SimpleRedis.
- Other windowcounter bugs.
- Reclaim table Sleep/Wake.
- A second stop channel instead of `stopping`.

## Decisions

1. **`stopping bool` on `Limiter` next to `closed`.** Set under `l.mu` in `stopFlushAndWait` before unlock (after `takeFlushTickerLocked`). Clear under `l.mu` after `wg.Wait` and `ticker.Stop`. Alternative: keep `stop` non-nil until after Wait — rejected; `flushLoop` needs the channel closed to exit. Alternative: `closed` during Sleep — rejected; that would make Wake after Sleep a no-op.

2. **If `takeFlushTickerLocked` returns nil, set then clear `stopping` before return (still under `l.mu`).** Wake after an already-stopped Sleep must still start. Alternative: leave `stopping` true — rejected; that permanently no-ops Wake.

3. **`Wake` returns if `closed || syncRate==0 || stop!=nil || stopping`.** Same lock as today. Alternative: Wake queues a restart after Sleep — rejected; Sleep wins means no new loop until Sleep returns; a later Wake still starts.

4. **Tests-first file `windowcounter/repro_sleep_wake_race_test.go` copied from the parent example** (`TestRepro_SleepWakeRaceHangs`, 80 iterations, 2s watchdog, 8 Wake workers, fake Redis helpers). Confirm FAIL on unfixed dest. Then implement. Sequential `TestWake_StartsTickerAfterSleep` in `limiter_test.go`. Keep `TestClose_StopsTickerAndKeepsRedis`.

## Risks / Trade-offs

- [Race test is probabilistic] → Mitigation: 80 iterations and 8 Wake workers; dest hung by iteration 2 in explore. Watchdog 2s so a hang fails the test, not the suite (`-timeout 60s`).
- [Wake during Sleep is dropped] → Mitigation: that is Sleep wins. Caller Wake after Sleep still starts.
- [Close and Sleep share `stopFlushAndWait`] → Mitigation: Close already sets `closed` first; Wake stays a no-op after Close even after `stopping` clears.

## Migration Plan

No signature change. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
