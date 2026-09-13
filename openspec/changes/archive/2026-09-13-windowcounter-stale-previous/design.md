## Context

Dest `takeBuffered` uses `windowLocked` for current (GET when `localDelta == 0`) and `bufferedCountLocked` for previous (memory if the key is in `l.windows`). After a window roll the previous Redis key is the same string that was current, so previous never GET-refreshes. See proposal.md for why. Specs: `std_go_windowcounter_sync-flush`, `std_go_windowcounter_sliding-take`. Explore: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Take-only previous GET when the key is in memory and `localDelta == 0`.
- Repro fails on dest code, then passes after the limiter change. Deny at estimated 3.

**Non-Goals:**
- GET previous on every Peek.
- INCR previous.
- Redis-down / `lastFlushErr` / missed-`sync_rate` probe.
- Live e2e two-client window-roll case.
- Calling `windowLocked` for previous with current `expireAt`.

## Decisions

1. **Take-only sibling, not `bufferedCountLocked`.** Peek uses `bufferedCountLocked`. Putting GET-when-`localDelta == 0` there would GET previous on every Peek after flush (skip storm). Alternative: parameter on `bufferedCountLocked` — rejected; Peek and Take would share a flag that hides two jobs. Alternative: `windowLocked(previousKey, expireAt)` — rejected; Take's `expireAt` is the current window and would rewrite previous's flush EXPIREAT.

2. **GET previous like `windowLocked` without writing `expireAt`, and only when `expireAt > 0`.** If the key is missing, GET-seed like today. If `localDelta == 0` and `expireAt > 0` (this instance counted it as current), GET and set `redisKnown`. If `localDelta > 0` or the key was only GET-seeded as previous (`expireAt == 0`), return memory. Failed GET returns from Take. Success sets `lastRedisOK`. Alternative: GET whenever `localDelta == 0` — rejected; that GETs previous on every same-window Take after first sight and fails dest `TestTake_BufferedPendingDeltaOutage`. Alternative: treat GET miss/failure as outage — rejected; ticket forbids folding into Redis-down.

3. **Repro file `windowcounter/repro_stale_previous_test.go` before any `limiter.go` edit.** Unit fake Redis, frozen clock (`start+9s` then `start+10s` for a 10s window), limit 2, A then B Take, A.Sleep then B.Sleep, A's next-window Take denies estimated 3. Alternative: fold into `limiter_test.go` first — rejected; caller named the repro file and implement order is fail-then-fix.

4. **Unit fake Redis only.** Dest e2e sliding boundary is exact-only. Alternative: add `limiter_e2e_test.go` buffered roll — rejected; explore assumed unit is enough.

## Risks / Trade-offs

- [Peek at the rolled clock can still see stale previous] → Mitigation: spec names the occupancy miss; Take is the refresh. Do not GET previous on Peek.
- [GET previous on every Take after flush adds one Redis round-trip] → Mitigation: same cadence as current `windowLocked`; not a skip-storm.
- [Holding `l.mu` across GET] → Mitigation: dest current path already does this; do not invent a second lock policy.

## Migration Plan

No signature change. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
