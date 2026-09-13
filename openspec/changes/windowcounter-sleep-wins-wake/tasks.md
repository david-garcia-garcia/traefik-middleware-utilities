## 1. Failing repro first

- [ ] 1.1 Add `windowcounter/repro_sleep_wake_race_test.go` with `TestRepro_SleepWakeRaceHangs` (parent example: fake Redis helpers, 80 iterations, 2s watchdog, 8 Wake workers)
- [ ] 1.2 Run `go test -short -count=1 -timeout 60s ./windowcounter -run TestRepro_SleepWakeRaceHangs` on unfixed dest and confirm FAIL (watchdog, not suite hang)

## 2. Stopping flag

- [ ] 2.1 Add `stopping bool` on `Limiter`. In `stopFlushAndWait`, set it under `l.mu` before unlock. If ticker is nil, clear it before return. Else `wg.Wait`, `ticker.Stop`, then clear under `l.mu`
- [ ] 2.2 `Wake` returns if `closed || syncRate==0 || stop!=nil || stopping`
- [ ] 2.3 Add `TestWake_StartsTickerAfterSleep` in `limiter_test.go`. Do not change `TestClose_StopsTickerAndKeepsRedis`

## 3. Prove

- [ ] 3.1 `TestRepro_SleepWakeRaceHangs` returns
- [ ] 3.2 `go test -short -count=1 -timeout 60s ./windowcounter` passes, including `TestClose_StopsTickerAndKeepsRedis`
- [ ] 3.3 Update `knowledge/devdocs/std_go_windowcounter.md` Gotchas: Sleep wins vs concurrent Wake; Wake after Sleep still starts; after Close do not start
- [ ] 3.4 `openspec validate --change windowcounter-sleep-wins-wake --strict`
