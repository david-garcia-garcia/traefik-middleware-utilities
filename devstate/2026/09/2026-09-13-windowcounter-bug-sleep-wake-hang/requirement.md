# Requirement
IssueKey: 2026-09-13-windowcounter-bug-sleep-wake-hang

## Problem
Concurrent `Wake` can hang `Sleep` forever. `stopFlushAndWait` sets `stop=nil` then unlocks then `wg.Wait`. `Wake` sees `stop==nil` and `startFlushLocked` (`wg.Add(1)` a new `flushLoop`). `Sleep` waits for a loop nobody will stop. Measured: hung iteration 0 after 2s.

## Current (code)
- `windowcounter/limiter.go` `Limiter` — `closed`, `ticker`, `stop`, `wg`. No `stopping` field.
- `windowcounter/limiter.go` `Sleep` — `flushPending` then `stopFlushAndWait`.
- `windowcounter/limiter.go` `Wake` — under `l.mu`, return if `closed || syncRate==0 || stop!=nil`; else `startFlushLocked`. Does not check a stopping flag.
- `windowcounter/limiter.go` `startFlushLocked` — `stop=make(chan struct{})`, `time.NewTicker`, `wg.Add(1)`, `go flushLoop`.
- `windowcounter/limiter.go` `takeFlushTickerLocked` — if `stop==nil` return nil; else `close(stop)`, `stop=nil`, detach `ticker`. Does not wait.
- `windowcounter/limiter.go` `stopFlushAndWait` — lock, `takeFlushTickerLocked` (clears `stop`), unlock, then `wg.Wait` and `ticker.Stop`. No flag set before unlock.
- `windowcounter/limiter.go` `Close` — `closed=true`, `flushPending`, `stopFlushAndWait`. Later `Wake` is a no-op because `closed`.
- `windowcounter/limiter_test.go` `TestClose_StopsTickerAndKeepsRedis` — sequential `Sleep`, `Close`, `Wake`; asserts `stop==nil` after that `Wake`. Redis client still usable. No concurrent `Sleep`/`Wake`.
- Dest `windowcounter/` has no `TestRepro_SleepWakeRaceHangs` and no `repro_sleep_wake_race_test.go`.
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` — Sleep flushes then stops the ticker; Wake SHALL start the ticker when `sync_rate` is greater than zero. Close after Sleep MUST NOT start a new ticker (scenario names Takes). No concurrent Sleep/Wake; no Sleep-wins rule.
- `knowledge/devdocs/std_go_windowcounter.md` — Sleep flushes then stops; after Close do not start a ticker. No concurrent Wake-during-Sleep hang.

## Desired
1. CREATE the failing repro FIRST (`TestRepro_SleepWakeRaceHangs`; example `windowcounter/repro_sleep_wake_race_test.go`). Confirm it hangs/fails on unfixed dest (2s watchdog; do not hang the whole run).
2. Then implement a `stopping` flag: set under `l.mu` in `stopFlushAndWait` BEFORE unlock; clear only after `wg.Wait` and `ticker.Stop`.
3. Sleep wins. `Wake` is a no-op if `closed || syncRate==0 || stop!=nil || stopping`.
4. `TestRepro_SleepWakeRaceHangs` must return. `go test -short -count=1 -timeout 60s ./windowcounter` passes.
5. Wake after Sleep still starts a ticker. After Close, Wake must not start a ticker. Keep `TestClose_StopsTickerAndKeepsRedis` behavior.

## Affected
- `windowcounter/limiter.go` (`Limiter`, `Wake`, `stopFlushAndWait`)
- `windowcounter/` tests (new failing-then-green race coverage; keep Close test)
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` (Sleep wins vs concurrent Wake)
- `knowledge/devdocs/std_go_windowcounter.md` (Sleep/Wake concurrent)

## Out of scope
- Other windowcounter bugs (peek, expire retry, fractional window, buffered outage, stale previous, lock during GET)
- Changing `TestClose_StopsTickerAndKeepsRedis` (Redis stays open; Wake after Close stays a no-op)
- reclaim table Sleep/Wake
- tokenbucket / simpleredis
- Exact mode (`sync_rate==0`) ticker (Wake already no-op)

## Unknowns
- Whether Wake-after-Sleep (without Close) is asserted in the repro file or a second test; ticket requires the behavior.
- Whether `stopping` is a bool on `Limiter` (ticket names the flag, not the type).
- Whether implement copies the parent example as-is or adapts names; dest must still expose `TestRepro_SleepWakeRaceHangs`.

## Tensions
- Ticket: Wake is a no-op while `stopping`. Spec `std_go_windowcounter_sync-flush` “Wake SHALL start the ticker when `sync_rate` is greater than zero” has no Sleep-in-progress exception. Ticket wins; after Sleep completes, Wake still starts a ticker. Spec/usage catch up in propose.
- Spec Close scenario says further Takes MUST NOT start a ticker. Existing test already asserts Wake after Close does not. Ticket keeps that; does not replace it with Takes-only.
- Caller example lives in the parent checkout, not dest. Tests-first on dest: land the failing repro, then the flag. Do not keep a hang as the post-fix contract.
