## Context

See proposal.md and `devstate/explore.md` options: dirty set, grouping by `windowStart`, grouping by `expireAt`, a `tokenbucket`-style recency list, a cap. A recency list does not order heterogeneous `expireAt` (one Limiter mixes window sizes, and `expireAt` only grows on bump). Grouping by `windowStart` breaks with mixed window sizes; grouping by `expireAt` does not. A cap bounds K but leaves the O(K) tick below it.

## Goals / Non-Goals

**Goals:**

- Idle tick cost independent of the number of live windows, compiled and Yaegi.
- Exactly the same flush outcomes and admission results.
- No pending delta can be lost; index consistency checkable in tests.

**Non-Goals:**

- A window cap or eviction policy.
- Changing batching (256 keys per EVAL), outage handling, or Redis scripts.

## Decisions

1. **Dirty index = map keyed by Redis window key.** `dirty map[string]*windowState`. In `takeBuffered`, before `localDelta++`, `if state.localDelta == 0 { l.dirty[key] = state }` (one map write only on the 0→1 transition, so hot keys pay nothing extra). `applyFlushOutcomeLocked`: after a successful apply, if `localDelta == 0 && flushDelta == 0`, `delete(l.dirty, key)`. A failed key keeps its delta and stays. Collect iterates `l.dirty` with the existing condition (`localDelta > 0 && flushDelta == 0`).

2. **Expiry index = buckets by `expireAt`.** `expiry map[int64]map[string]*windowState`. Two helpers own all membership changes: `putWindowLocked(key, state)` (insert into `l.windows` and its bucket) and `setExpireLocked(key, state, newExpire)` (move between buckets only when the value changes). Every current `state.expireAt = …` and every insert into `l.windows` in non-test code is replaced by these helpers. Deletion goes through `deleteWindowLocked(key, state)` (map, bucket, dirty).

3. **Prune visits due buckets only.** For each bucket second `e` with `e == 0 || nowUnix >= e`: for each window in it, delete it when `localDelta == 0 && flushDelta == 0` (same condition as today); keep it otherwise; drop the bucket when empty. Iterating the bucket map is O(distinct seconds), which stays small because `expireAt = windowStart + 2*window` aligns to epoch boundaries. Alternative: a sorted slice or heap of seconds — rejected, more code for a set that is already tiny.

4. **Test helpers stay honest.** Test code that inserts into `l.windows` directly is migrated to `putWindowLocked`/`setExpireLocked`, so no test can build a state the indexes do not know. Add a test-only `checkIndexesLocked` (in a `zzz_` file) that asserts: every window is in exactly one bucket matching its `expireAt`; `dirty` equals the set of windows with `localDelta > 0` (or `flushDelta > 0` while a flush is in flight).

5. **Tests and benchmarks.** `windowcounter/zzz_flush_dirty_index_test.go`, helpers suffixed `FlushDirtyIndex`: (a) randomized sequence of buffered Takes/Peeks on many keys and window sizes, flushes with injected per-key failures, clock advances across window boundaries; after each step `checkIndexesLocked` holds and, at the end, Redis counts equal the exact number of Takes per window (no lost or doubled delta beyond the documented ambiguous-retry case); (b) idle tick scaling: median wall time of `flushPending` with 200k idle non-due windows is below 3× the median with 10k (O(K) is ~20×) — compiled; (c) existing flush, outage, stale-previous, and batch tests unchanged. Benchmarks `BenchmarkFlushIdle/{10k,200k}` compiled and the Yaegi equivalent; record both in the delivery card next to the explore baseline.

## Risks / Trade-offs

- [Every `expireAt` write site must use the helper; a missed site leaves a stale bucket] → `checkIndexesLocked` in the randomized test catches it; grep for `.expireAt =` in non-test code must only show the helper.
- [Extra memory: two map entries per window] → roughly doubles per-window index overhead; acceptable versus the mutex stall, and the cap debt still applies.
- [Yaegi cost on the Take path] → one extra map write only on 0→1 transitions and on `expireAt` changes; measure warm Take compiled and Yaegi (existing benchmarks) and report.

## Migration Plan

Internal change. Rollback is revert.

## Open Questions

None.
