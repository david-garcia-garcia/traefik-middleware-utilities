## Context

See proposal.md and `devstate/explore.md` options: (1) generation/owner of `stopping`, (2) stop refcount, (3) serialize `stopFlushAndWait` with a dedicated mutex, (4) a nil-timer caller waits without clearing. (3) leaves a gap between one stopper's unlock and the next one's lock where Wake can arm. (1) and (4) need extra state to know whether a stop is pending. (2) expresses "a stop is pending" directly.

## Goals / Non-Goals

**Goals:**

- No Sleep returns before the in-flight flush completes.
- No timer chain starts while any stop is pending; at most one chain exists afterwards; every Sleep returns.
- Keep the `time.AfterFunc` model; no interpreted `select`.

**Non-Goals:**

- Changing Close semantics, flush semantics, or reclaim.

## Decisions

1. **Stop counter.** Replace `stopping bool` with `stoppers int`. `stopFlushAndWait`: lock; `stoppers++`; `timer := flushTimer; flushTimer = nil`; unlock; `if timer != nil && timer.Stop() { flushBusy.Done() }`; `flushBusy.Wait()`; lock; `stoppers--`; unlock. Every caller waits, including one that found no timer. Alternative (3) — rejected, Wake can slip between two serialized stoppers. Alternative (4) — rejected, it needs to know a stop is in flight, which is the counter.

2. **Arming checks.** `Wake` arms only when `!closed && syncRate > 0 && flushTimer == nil && stoppers == 0`. `flushAfterTick` re-arms only when `!closed && stoppers == 0 && flushTimer != nil` (the existing condition with the counter). Because `Add(1)` happens only with `stoppers == 0`, no `Wait` is in progress at that moment.

3. **Tests.** `windowcounter/zzz_concurrent_sleep_stop_test.go`, helpers suffixed `ConcurrentSleepStop`, black-box with the package fake Redis holding the next flush EVAL: (a) two concurrent Sleeps while the EVAL is held — neither returns before the EVAL is released; both return after; (b) same plus a Wake while both wait — after release both Sleeps return within a bound, no timer is armed while they wait, and a final Wake then Sleep arm and stop exactly one chain; (c) control: single Sleep/Wake race test from the live spec still passes; (d) run (a) under `-race`. Use deadlines, not sleeps, to assert "has not returned yet".

## Risks / Trade-offs

- [A Sleep can now wait for another caller's in-flight flush] → That is the intended contract: Sleep promises the flush finished.

## Migration Plan

Internal change. Rollback is revert.

## Open Questions

None.
