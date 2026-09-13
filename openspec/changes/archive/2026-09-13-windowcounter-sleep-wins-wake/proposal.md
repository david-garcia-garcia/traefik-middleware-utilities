## Why

On DestBranch, concurrent `Wake` can hang `Sleep` forever. `stopFlushAndWait` clears `stop` and unlocks, then `wg.Wait`. `Wake` sees `stop==nil` and `startFlushLocked` (`wg.Add(1)` a new `flushLoop`). Sleep waits for a loop nobody will stop. Measured on dest: `TestRepro_SleepWakeRaceHangs` hung iteration 2 after 2s.

## What Changes

- Land `TestRepro_SleepWakeRaceHangs` first (`windowcounter/repro_sleep_wake_race_test.go`). Confirm it fails on unfixed dest with a 2s watchdog.
- Add a `stopping` bool on `Limiter`. `stopFlushAndWait` sets it under `l.mu` before unlock; clears it only after `wg.Wait` and `ticker.Stop` (or on the already-stopped early return).
- Sleep wins: `Wake` is a no-op if `closed || syncRate==0 || stop!=nil || stopping`.
- After Sleep completes, Wake still starts a ticker. After Close, Wake must not. Keep `TestClose_StopsTickerAndKeepsRedis`. Add `TestWake_StartsTickerAfterSleep`.
- Spec `std_go_windowcounter_sync-flush` and usage `knowledge/devdocs/std_go_windowcounter.md` name Sleep-wins vs concurrent Wake.

## Capabilities

### New Capabilities

- None. Fold into the existing Sleep/Wake/Close ticker leaf.

### Modified Capabilities

- `std_go_windowcounter_sync-flush`: Sleep wins against concurrent Wake. Wake SHALL NOT start a ticker while Sleep or Close is waiting for the flush loop. After Sleep returns, Wake still starts the ticker when `sync_rate` is greater than zero. After Close, Wake still MUST NOT start a ticker.

## Impact

- `windowcounter/limiter.go` (`Limiter.stopping`, `Wake`, `stopFlushAndWait`).
- `windowcounter/repro_sleep_wake_race_test.go` (new), `windowcounter/limiter_test.go` (`TestWake_StartsTickerAfterSleep`; keep Close test).
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` (after archive).
- `knowledge/devdocs/std_go_windowcounter.md` Sleep/Wake gotcha.
- No reclaim table. No tokenbucket. No other windowcounter bugs. No Close of the injected SimpleRedis.
