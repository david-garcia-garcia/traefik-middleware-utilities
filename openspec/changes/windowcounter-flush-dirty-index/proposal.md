## Why

In buffered mode every flush tick, `Sleep`, and `Close` walk the whole `l.windows` map twice under `l.mu` (collect dirty windows, then prune expired ones), even when nothing is dirty. Measured on dest for an idle tick: 11.1 ms compiled and 51.9 ms under Yaegi at 200k live windows, 67 ms compiled at 1M. The timer re-arms after the flush, so at `syncRate` 20 ms and 200k windows the mutex is busy ~72 % of the time under Yaegi and every Take/Peek of that Limiter, for any key, waits behind the scan. Per-IP limiting under IP rotation sets the window count.

## What Changes

- A **dirty index** (`dirty map[string]*windowState`) holds exactly the windows with `localDelta > 0`. A window joins it when its `localDelta` goes from 0 to positive in `takeBuffered`, and leaves it when a successful flush brings `localDelta` and `flushDelta` back to 0. The collect step iterates the dirty index, not `l.windows`.
- An **expiry index** (`expiry map[int64]map[string]*windowState`) buckets every window by its `expireAt` second (bucket 0 for windows seeded with `expireAt` 0). Every write of `expireAt` and every insert/delete in `l.windows` goes through one helper that keeps the bucket membership. The prune step visits only buckets whose second has passed (and bucket 0), deletes idle windows from them, keeps windows that still carry a delta, and drops empty buckets.
- Tick cost becomes O(dirty windows + windows due now + number of distinct `expireAt` seconds), which is small because windows align to epoch boundaries.
- Admission results, flush semantics (`std_go_windowcounter_sync-flush`), batching, and the retained-outage behavior do not change. No delta can be lost: a window with `localDelta > 0` or `flushDelta > 0` is never deleted.
- The map cap stays out of scope (`knowledge/debt/2026-09-13-windowcounter-outage-windows-unbounded.md`).

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- None. No live promise changes (`devstate/specs.md`: none — no live contract).

## Impact

- `windowcounter/limiter.go`: `Limiter` fields, `New`, every insert into `l.windows` (`applyGetLocked`, `seedUnreadWindowLocked`), every `expireAt` write (`peekCountLocked`, `windowLocked`, `applyGetLocked`, `seedUnreadWindowLocked`, others found by grep), `takeBuffered`, `applyFlushOutcomeLocked`, `flushPendingLocked`.
- Tests that build `l.windows` by hand (`zzz_limiter_flush_batch_test.go` `windowStateForTest`, bench helpers `DirtyWindowKeysForBench` / `RedirtyWindowsForBench`) must go through the same helper.
- New tests and compiled/Yaegi idle-tick benchmarks.
- Sibling `windowcounter-concurrent-sleep-stop` edits `stopFlushAndWait`/`Wake` in the same file; hunks do not overlap.
