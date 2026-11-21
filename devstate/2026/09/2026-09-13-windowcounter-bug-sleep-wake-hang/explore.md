# Explore
IssueKey: 2026-09-13-windowcounter-bug-sleep-wake-hang

## Concepts

Sleep / Wake / Close are `reclaim.Hooks` on `windowcounter.Limiter`. Buffered mode (`sync_rate > 0`) owns one `time.Ticker` and one `flushLoop` goroutine. `stop` is both “a loop is running” and the close-to-exit signal. `takeFlushTickerLocked` closes `stop`, sets `stop = nil`, detaches `ticker`, and does not wait. `stopFlushAndWait` then unlocks and `wg.Wait`s.

```
Sleep                         Wake
  flushPending
  lock
  close(stop); stop=nil  ──►  lock
  unlock                      stop==nil → startFlushLocked
  wg.Wait  ◄──── Add(1) new flushLoop
  (never returns)
```

Dest `windowcounter/limiter.go` has `closed`, `ticker`, `stop`, `wg`. No `stopping`. `Wake` returns if `closed || syncRate==0 || stop!=nil`. After Close, `closed` already makes Wake a no-op (`TestClose_StopsTickerAndKeepsRedis`).

Usage `knowledge/devdocs/std_go_windowcounter.md` and spec `std_go_windowcounter_sync-flush` say Sleep stops the ticker and Wake starts it when `sync_rate > 0`. Neither names a Sleep-in-progress exception. Ticket wins: Sleep wins while stopping; after Sleep completes, Wake still starts.

Reclaim’s own Sleep/Wake are test doubles. Out of scope.

No third-party identity owner. No research write.

## Decisions

- Reproduce on dest, do not implement in explore. Copied parent `repro_sleep_wake_race_test.go` into dest, ran `go test -short -count=1 -timeout 30s ./windowcounter -run TestRepro_SleepWakeRaceHangs`, deleted the file. **FAIL** iteration 2 after 2s (`Sleep hung on iteration 2 after 2s`). Ticket measured iteration 0; same hang, watchdog returned. Dest has the bug.
- Sleep wins via a `stopping bool` on `Limiter`. Set under `l.mu` in `stopFlushAndWait` before unlock (after `takeFlushTickerLocked` so `stop` is already nil). Clear only after `wg.Wait` and `ticker.Stop`, under `l.mu` again. If `takeFlushTickerLocked` returned nil (already stopped), set then clear `stopping` on that same locked section so Wake-after-Sleep still starts. Do not leave `stopping` true on the early return.
- `Wake` no-op if `closed || syncRate==0 || stop!=nil || stopping`.
- Tests-first in implement: land `windowcounter/repro_sleep_wake_race_test.go` with `TestRepro_SleepWakeRaceHangs` (caller example; dest has no `repro_*` yet; name is the hang). Confirm fail on unfixed dest with the 2s watchdog. Then the flag. Keep `TestClose_StopsTickerAndKeepsRedis`. Add `TestWake_StartsTickerAfterSleep` in `limiter_test.go` (sequential Sleep then Wake, `stop != nil`) — not the race file. Close path stays the existing test.
- Spec/usage catch up in propose: Sleep-wins vs concurrent Wake; Wake after Sleep still starts; Close still forbids a new ticker. Do not rewrite exact/buffered/Peek requirements.
- Bound: only this race. No peek/expire/fractional/buffered-outage/stale-previous/lock-during-GET. No reclaim table. No tokenbucket.

## Open questions

- Q: Is `stopping` a bool on `Limiter`?
  Rank: additive asked — new field this change creates; requirement Desired names the flag
  Decision: assumed — bool on `Limiter` next to `closed` / `stop`; ticket named the flag not a channel.
  By: explore

- Q: Where is Wake-after-Sleep (no Close) asserted?
  Rank: additive asked — new test this change creates; requirement Desired names the behavior
  Decision: assumed — `TestWake_StartsTickerAfterSleep` in `windowcounter/limiter_test.go` beside `TestClose_StopsTickerAndKeepsRedis`. Race file only proves Sleep returns under concurrent Wake.
  By: explore

- Q: Copy parent `repro_sleep_wake_race_test.go` as-is?
  Rank: additive asked — new test file; requirement Desired names `TestRepro_SleepWakeRaceHangs` and the example path
  Decision: assumed — same package, helpers (`startTestFakeRedis`, `newSimpleRedisForTest`), 80 iterations, 2s watchdog, 8 Wake workers. Keep the test name. After the flag it must return, not stay a hang contract.
  By: explore

- Q: If `stopFlushAndWait` finds no ticker, do we still set `stopping`?
  Rank: additive asked — same flag the criterion names; already-stopped Sleep must not permanently no-op Wake
  Decision: assumed — set under the lock before unlock; if ticker is nil, clear `stopping` before return (still under `l.mu`). Clear after Wait+Stop only when a ticker was taken.
  By: explore
