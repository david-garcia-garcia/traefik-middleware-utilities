## Context

Dest `windowcounter.Limiter` already admits from `redis_known + local_delta`, then returns `bufferedOutageErrorLocked` (stored `lastFlushErr`, else EVAL/GET after one missed `sync_rate`). `windowLocked` GETs when `localDelta == 0` and returns that GET error. Exact INCR still returns Redis errors. Fake `Kill` already closes the listener and accepted sockets. See proposal.md for why. Specs: `std_go_windowcounter_sliding-take`, `std_go_windowcounter_sync-flush`. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Buffered Take/Peek outage: local admit/deny, `err=nil`, this node's `limit`.
- GET error on seed or share-refresh uses the existing buffer (0 on first sight) and remembers the outage so later calls skip Redis.
- Tests exist that fail on dest's fail-closed lock, then pass after the hunk.

**Non-Goals:**
- Fail-closed `takeBuffered` (do not keep or add returning a Redis error as the buffered-outage result).
- GET or INCR on every buffered Take to probe.
- Returning `redis:unreachable` because `localDelta > 0` skipped GET.
- Exact-mode error propagation.
- Bugs 2–7.
- Parent `trackConn` / `closeListenerAndConns` names.
- Deleting unread `flushFailedAt`.
- Reconstructing client identity (caller owns the opaque key).

## Decisions

1. **Tests first, rewrite the lock to nil error + local deny.** Copy/adapt parent `windowcounter/repro_buffered_outage_test.go`. Assert `err=nil`, admit until this node's `limit`, then `allowed=false`. Dest `Kill` only. Rewrite dest cases that still `wantRedisOutage` (`TestTake_BufferedPendingDeltaOutage`, `TestTake_BufferedFlushThenKillFailsClosed`, `TestTake_BufferedTwoInstancesOutage`, `TestTake_BufferedSleepStoresFlushError`, `TestPeek_BufferedEmptyFlushThenKill`). Keep `TestTake_Unreachable` / `TestPeek_Unreachable`. Alternative: keep fail-closed assertions as the post-fix contract — rejected; Desired 1 forbids it.

2. **On buffered GET error, use the existing buffer and `err=nil`.** Seed 0 on first sight. Keep `redisKnown + localDelta` when the window is already in memory. Store the outage on the existing `lastFlushErr` so later Take/Peek skip share-refresh GET instead of paying timeout every call. Alternative: return the GET error — rejected; that is dest fail-closed (`TestTake_BufferedFlushThenKillFailsClosed`). Alternative: a second outage boolean beside `lastFlushErr` — rejected; Consume before produce.

3. **Disconnect Take/Peek from returning `lastFlushErr` and from the missed-`sync_rate` EVAL/GET probe.** Keep writing `lastFlushErr` in `flushPendingLocked` (Sleep/Close/flushLoop already call it). Remove `bufferedOutageErrorLocked` as an error-returning helper. Do not change `takeBuffered` to fail-closed. Alternative: keep returning the probe error — rejected; Desired 4.

4. **Peek follows Take.** Same nil-error local observation, no hit. Language already defines Peek as that observation; dest `peekBuffered` already shares the outage helper. Alternative: leave Peek fail-closed — rejected; that splits the sibling.

5. **Exact Take/Peek still return Redis errors.** No change to `takeExact` / `peekExact`.

6. **Identity is the caller's opaque key.** Do not reconstruct client address, user, tenant, Host, or trust hop.

## Risks / Trade-offs

- [N instances admit about `limit × N` while Redis is down] → Mitigation: that is the accepted per-node cap; document it on usage/README after the code is true.
- [Share-refresh GET skipped while `lastFlushErr` is set, so Peek-only may not see Redis recover until a Take flushes] → Mitigation: recovery is the flush ticker when a delta exists; do not probe on Take/Peek.
- [Callers that check `err` to fail closed after PR #30 will now see nil] → Mitigation: **BREAKING** is the ticket; document the deviation.

## Migration Plan

No signature change. Middleware that treated buffered-outage `err` as fatal will now see the local admit. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
