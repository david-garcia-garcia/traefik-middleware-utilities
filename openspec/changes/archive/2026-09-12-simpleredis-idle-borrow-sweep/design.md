## Context

Dest `takeIdleConn` pops the idle tail and returns on the first still-young socket; `release` only appends. `borrow` already closes the `stale` slice after unlock. Fake `openSockets` / `waitOpenSocketsEqual` and `holdGetsForTest` exist. See proposal.md for why. Proceed policies: `devstate/explore.md`. OPEN PR 12 peels heads on `release` and is not this change.

## Goals / Non-Goals

**Goals:**
- Full-list sweep in `takeIdleConn`: keep LIFO reuse of the newest survivor; close stale after the lock.
- Fake two-age proof on `openSockets`. Unchanged goroutine count across `New`.

**Non-Goals:**
- Background reaper or `reclaim.Hooks{Close}`.
- Peel-on-release (PR 12).
- bug-03 `MaxIdleConns` independent of `liveCap()`.
- Live Redis/Dragonfly idle-head tests.
- Changing `IdleTimeout` / `PoolSize` / `MaxIdleConns` defaults.

## Decisions

1. **Sweep the whole idle slice in `takeIdleConn`.** Compact still-young sockets in place, collect stale, pop the newest survivor. Alternative: peel stale heads on `release` (PR 12) — rejected; a two-age borrow with no later release still leaves the head established. Alternative: background ticker — rejected; dest product does not call `Close`.

2. **Close stale after unlock.** Dest `borrow` already does this for the tail-pop stale slice. Do not `close()` under `idleConnsMu`.

3. **Two overlapping Gets, then age only `idleConns[0]`.** Use `holdGetsForTest` so two sockets sit idle, backdate the head `lastUsed`, one `Get`, `waitOpenSocketsEqual` for one remaining open socket, `connections()` still 2. Alternative: assert `len(idleConns)` only — rejected; a leak that forgets without `Close` would pass. Count `runtime.NumGoroutine()` immediately before and after `New` in that test (or a sibling).

## Risks / Trade-offs

- [Fully quiet client never borrows, sockets stay] → Mitigation: recorded as `knowledge/debt/2026-09-12-simpleredis-idle-reaper-reclaim.md`; not this change.
- [O(idle) under `idleConnsMu`] → Mitigation: idle length is at most `PoolSize` in the default config (8); bug-03 is out of scope.
- [`NumGoroutine` is racy] → Mitigation: sample immediately around `New`; do not sleep; sweep adds no goroutine.

## Migration Plan

Library behavior change. Rollback is revert. No deploy contract.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
