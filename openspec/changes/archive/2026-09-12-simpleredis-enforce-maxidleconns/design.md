## Context

DestBranch `release` closes only when idle is already at `maxIdleConns` **and** `len(idleConns)+inUse >= liveCap()`, and `inUse` still includes this socket because `freeInUseTurn` runs after the check. Config, usage, and the spec's "at most MaxIdleConns" sentence already describe idle-only trim. go-redis `Put` closes when idle is full even under `PoolSize` (`knowledge/research/ext_go-redis_connection-pool/`). Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Idle-only close in `release`.
- Same-package tests that DestBranch currently fails (table + concurrent quiesce + still-open sockets).
- Spec sentence matches Config.

**Non-Goals:**
- Clamp `MaxIdleConns` to `PoolSize` in `applyDefaults`.
- Idle reaper / `ConnMaxIdleTime`.
- Changing `PoolSize` or the in-use-turn semaphore.
- A new peak-idle sampler from the external audit harness.

## Decisions

1. **Idle list is the only trim predicate.** Close when `closed` or `len(idleConns) >= maxIdleConns`. Delete `inUse` / `live` from `release`. Keep `liveCap()` for `PoolSize()`. Alternative: wait until `freeInUseTurn` then re-check live — rejected; that still ties idle trim to `PoolSize` and the ticket asks to drop the live term.

2. **Same-package `borrow`/`release` table, not only Get.** Unexported `idleConns` is visible in `simpleredis/*_test.go`. Table `8/2`, `16/1`, `2/8`, `8/8`. Idle after each return is the peak series. Alternative: public API only — cannot read `idleConns` from another package.

3. **Still-open proof is `waitOpenSocketsEqual`.** `hangupCount` can increment before `open--`. Do not assert `connections()-hangupCount()` as still-open. Alternative: immediate `openSockets()` snapshot — races TCP close.

4. **Replace `TestReleaseKeepsSocketWhenLiveUnderCap`.** That test requires idle above default 8. Keep a reuse Get that does not increase accepts when idle is at the cap. Concurrent path: overlapping Gets with PoolSize 16 / default MaxIdleConns 8, then idle 8 and `waitOpenSocketsEqual(8)`.

5. **No usage rewrite unless the gotcha is wrong after apply.** `knowledge/devdocs/std_go_simpleredis.md` already says idle trim is `MaxIdleConns`.

## Risks / Trade-offs

- [Bursts wider than MaxIdleConns redial next wave] → Mitigation: that is the Config contract; go-redis does the same when MaxIdleConns is set. Out of scope: documenting reconnect-vs-footprint (`perf-01`).
- [A fix that only shrinks `idleConns` without `Close`] → Mitigation: `waitOpenSocketsEqual` (decision 3).
- [MaxIdleConns > PoolSize never trims] → Mitigation: idle cannot exceed PoolSize because dials wait on the in-use-turn semaphore; table `2/8` asserts idle equals PoolSize.

## Migration Plan

Library behavior change for operators who already set `MaxIdleConns` below `PoolSize`: extra idle sockets start closing. Default both 8: sequential load unchanged. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
