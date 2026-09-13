# Requirement
IssueKey: 2026-09-13-windowcounter-bug-stale-previous

## Problem
Buffered Take never GET-refreshes the previous-window key once it is in `l.windows`. After A flushes then B flushes, Redis holds both hits but A's `redisKnown` for that key stays A's flush return. At the next-window start (weight=1) A's Take admits (`estimated=2`) instead of deny at 3.

## Current (code)
- `windowcounter/limiter.go` `bufferedCountLocked` — if the Redis key is already in `l.windows`, returns `redisKnown + localDelta` with no GET.
- `windowcounter/limiter.go` `takeBuffered` — current via `windowLocked`; previous via `bufferedCountLocked`; then `localDelta++` on current only.
- `windowcounter/limiter.go` `windowLocked` — GET current when the key is unseen, or when it is in memory and `localDelta == 0`; writes `redisKnown`.
- `windowcounter/limiter.go` `peekBuffered` — previous also via `bufferedCountLocked` (same no-GET-if-buffered path).
- `windowcounter/limiter.go` `peekCountLocked` — current Peek GETs only on first sight; comment forbids GET just because `localDelta` is 0.
- `windowcounter/limiter.go` `flushPendingLocked` — EVAL INCRBY on `localDelta > 0`; on success `redisKnown` becomes the EVAL return (this instance's post-flush total for that key). Sleep order A then B leaves A's previous `redisKnown` at 1 while Redis is 2.
- `windowcounter/limiter.go` `takeExact` — INCR current, GET previous every Take. Not this bug's path.
- `windowcounter/limiter_test.go` `TestTake_SlidingBoundaryDoesNotDouble` — exact mode (`syncRate` 0) only.
- `windowcounter/limiter_test.go` `TestBuffered_TwoClientsShareWithoutLastWriteWins` — two clients, same window after Sleep; `windowLocked` GETs current after flush. Does not roll onto the next window.
- `windowcounter/limiter_e2e_test.go` `slidingBoundary` — exact mode (`New(client, 0)`).
- `windowcounter/limiter_e2e_test.go` `bufferedTwoClients` — same-window share after Sleep, not a window roll.
- Dest `windowcounter/` has no `repro_stale_previous_test.go` and no buffered next-window previous-GET case.
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` — at next-window start the estimate still includes previous hits; must not admit a second full limit.
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` — after both flush, a later Take sees the shared count; last flush must not overwrite the other client. Buffered Peek must not GET every call because `local_delta` is 0.
- `knowledge/devdocs/std_go_windowcounter.md` — sliding estimate is current + previous × weight; buffered Peek is memory after first sight.

## Desired
1. Failing repro first, then `limiter.go` fix. Test must deny at estimated 3 after A.Sleep then B.Sleep and next-window Take (current=1 + previous=2 × weight=1).
2. On buffered Take, if the previous key is in memory and `localDelta == 0`, GET and set `redisKnown`. If `localDelta > 0`, keep memory (Redis does not have this instance's unflushed delta).
3. Do not INCR previous.
4. Do not GET previous on every Peek.
5. Do not fold into Redis-down / pending-delta outage policy.

## Affected
- `windowcounter/limiter.go` (`bufferedCountLocked` Take path, or Take-only refresh of previous)
- `windowcounter/` tests (new failing-then-green boundary case)
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` if buffered previous GET-when-`localDelta==0` is not already the written cadence
- `knowledge/devdocs/std_go_windowcounter.md` if usage still implies previous is only first-sight

## Out of scope
- Other windowcounter bugs (buffered outage, Sleep/Wake hang, EXPIRE retry, lock across GET, fractional window, Peek occupancy)
- INCR previous
- GET previous on every Peek
- Changing Redis-down / `localDelta > 0` skip-GET policy
- Exact-mode Take/Peek (already GET previous every call)

## Unknowns
- Whether Take-only previous GET lives in `bufferedCountLocked` (would change Peek too) or a Take-only sibling so Peek stays skip-storm.
- Whether a failed GET of previous on buffered Take propagates like `windowLocked` (current already does) or is treated as outage policy. Ticket says do not fold into Redis-down; Redis is up in the repro.
- Test file name vs landing in `limiter_test.go`.
- Whether live e2e must add a buffered two-client window-roll case, or unit fake Redis is enough this change.

## Tensions
- Spec `std_go_windowcounter_sliding-take` "dump at the window boundary does not double" vs dest buffered Take: previous stays this node's last flush. Ticket wins; dest exact tests already pass.
- Spec `std_go_windowcounter_sync-flush` "later Take sees the shared count" is proven only in the same window (`TestBuffered_TwoClientsShareWithoutLastWriteWins`). Ticket extends that to the rolled previous key.
- Spec "Peek agrees with Take before the increment" vs ticket "do not GET previous on every Peek": Peek at the boundary before a refreshing Take can still see stale previous. Honour no-Peek-GET; Take is the refresh. Explore owns whether Peek-then-Take at that clock is a documented occupancy miss.
- Spec buffered Peek GET cadence "same as buffered Take" vs ticket Peek stays `bufferedCountLocked`. Ticket: Peek must not GET previous every call; Take refreshes previous like current.
