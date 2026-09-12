## Context

DestBranch `windowcounter.Limiter` already has exact INCR Take, buffered `redis_known`/`local_delta`, `flushPending`, `SetNowForTest`, and Peek. `flushLoop` / `Sleep` / `Close` discard `flushPending`'s error. `windowLocked` GETs only when `localDelta == 0`. See proposal.md for why. Specs: `std_go_windowcounter_sliding-take`, `std_go_windowcounter_sync-flush`. Explore: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- One error surface: retained flush error plus staleness k=1 probe via flushPending.
- Take/Peek keep `(bool, float64, error)`.
- Killable fake and the four proofs.

**Non-Goals:**
- `LastFlushError()` / `Stale()` poll API.
- Fourth return value or typed admit+error wrapper type.
- GET on every buffered Take.
- Exporting `simpleredis` sentinel vars.
- Other `simpleredisfixes2` findings.

## Decisions

1. **Store `lastFlushErr`, `flushFailedAt`, `lastRedisOK` on `Limiter` under `l.mu`.** Set them in `flushLoop`, `Sleep`, and `Close` from `flushPending`. Success clears `lastFlushErr` and sets `lastRedisOK`. Seed GET success sets `lastRedisOK`. Alternative: poll-only methods — rejected; proof tests require Take's error to be non-nil and the live spec forbids a silent fallback.

2. **Staleness k=1, probe with flushPending.** If `lastFlushErr != nil`, Take/Peek return it. Else if `now.Sub(lastRedisOK) >= syncRate`, call `flushPending` once and return that error. Do not GET every buffered Take. Alternative: k=2 — rejected; one missed interval is the smallest bound named in explore. Alternative: homemade stale error with no Redis contact — rejected; that is a health-gate.

3. **Buffered Take still increments and still returns allowed/estimate beside the error.** Middleware that checks `err` first fails closed. Middleware that ignores `err` can fail-open from the local admit. Exact mode stays `false, 0, err` after the Redis call fails. Alternative: skip the increment on error — rejected; that removes the admit decision the ticket asked to keep.

4. **Peek uses the same lastFlushErr / staleness path.** Spec Peek unreachable SHALL. Alternative: Take only — rejected; that leaves the silent sibling.

5. **Killable fake: close listener and every accepted socket.** Track live conns on `testFakeRedis`. `Kill` closes the listener then each conn. Tests use long `syncRate` plus `SetNowForTest` to expire staleness without a real tick.

6. **`parseEvalInt` wraps `strconv.ParseInt` with `fmt.Errorf("%s: %w", simpleredis.RedisIssue, convErr)`.** Same for the empty-reply branch where there is no convErr (`errors.New` stays). Do not export sentinels (api-01).

## Risks / Trade-offs

- [Probe flushPending on Take holds `l.mu` and talks to Redis] → Mitigation: only after k=1; hot path between ticks is still memory. Same lock as today's flush.
- [Fail-open callers that ignore err still over-admit locally up to `limit`] → Mitigation: that is the explicit middleware choice; the error is no longer nil.
- [Sleep flushes then stops the ticker; tests must not use Sleep as the only probe] → Mitigation: `SetNowForTest` for proof 1; failed flushLoop covers stored `lastFlushErr`.
- [Two-instance test races the ticker] → Mitigation: long `syncRate` plus clock inject; assert nil-error admits, not wall time.

## Migration Plan

No signature change. Callers that already treat Take/Peek errors as fatal fail closed in buffered outage, matching exact mode. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
