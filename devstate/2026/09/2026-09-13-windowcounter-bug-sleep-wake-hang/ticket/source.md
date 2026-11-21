# Concurrent Wake can hang Sleep

Concurrent Wake can hang Sleep. stopFlushAndWait sets stop=nil then unlocks then wg.Wait. Wake sees stop==nil and startFlushLocked (wg.Add(1) new flushLoop). Sleep waits forever. Measured: hung iteration 0 after 2s.

Agreed how: Sleep wins. stopping flag set under l.mu in stopFlushAndWait BEFORE unlock, cleared only after wg.Wait and ticker.Stop. Wake is no-op if closed || syncRate==0 || stop!=nil || stopping. TestRepro_SleepWakeRaceHangs must return. Wake after Sleep still starts a ticker. After Close, Wake must not start a ticker.

Implement order (required):
1. CREATE the failing repro FIRST. Example: `d:\repositories\traefik-middleware-utilities\windowcounter\repro_sleep_wake_race_test.go`. Confirm it hangs/fails on unfixed code (use the 2s watchdog, do not hang the whole run).
2. Then implement the stopping flag.
3. Confirm the race test RETURNS and `go test -short -count=1 -timeout 60s ./windowcounter` passes. Keep TestClose_StopsTickerAndKeepsRedis behavior.

Bound the ask: only this bug.
