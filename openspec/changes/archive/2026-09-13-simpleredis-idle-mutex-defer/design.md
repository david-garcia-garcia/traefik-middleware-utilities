## Context

Dest `release` locks `idleConnsMu` and unlocks by hand on two paths, then `conn.close()` / `freeInUseTurn` after unlock. Dest `takeIdleConn` already `Lock` + `defer Unlock`. Dest `Close` already closes sockets outside the lock. Session source is stdlib-only, no generics, Yaegi workarounds kept. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- `defer` owns `idleConnsMu` unlock inside the keep-or-close owner.
- Unlock does not span `conn.close()` or `freeInUseTurn`.
- `freeInUseTurn` last on every `release` path.

**Non-Goals:**
- A production seam to panic inside the locked section.
- Changing Close, `takeIdleConn` stale-after-lock close, `errors.Is`, AfterFunc, `math/rand`, or `OverFrees` itself.
- Rebasing onto master while #71 is open.

## Decisions

1. **Extract `parkIdleConn` (sibling of dest `takeIdleConn`).** It locks, `defer`s unlock, computes `idleConnsFull` / `inUse` / `live` (turn still held), appends on success, returns bool. `release` stamps `lastUsed` on the reusable path, then `if !reusable || !sr.parkIdleConn(conn) { conn.close() }`, then `freeInUseTurn()`. Alternative: `defer Unlock()` at the top of `release` — regression; mutex held across close and the channel send. Alternative: `recover` in `release` — second recover next to Traefik; Yaegi may convert the panic to an Eval error so recover would not run; defer still runs.

2. **No new panic-inside-lock test.** Existing `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestOverFreeOnFullSemaphoreReturns`, `TestGetCancelFreesTurnAndDoesNotPool`, `TestPanicInDoReturnsTurnAndClosesSocket` cover the four frozen semantics. A panic inside the locked section would need a production hook (forbidden).

3. **`parkIdleConn` MUST NOT call `freeInUseTurn` or `conn.close`.** Those stay in `release` after return so the lock is dropped first and there is exactly one free per `release`.

4. **Task B is two `defer`s on `groupWriteMu`.** Nothing in those getters/setters can panic; consistency only.

5. **Task C rewrites BUGS.md section 2 to the Yaegi-defer finding** and points at dest `simpleredis/yaegi_defer_test.go`. Dest usage packet and tcp-session spec already have that finding; do not duplicate unless a stale sentence remains.

## Risks / Trade-offs

- [Mechanical conflict with OPEN `2026-09-13-simpleredis-close-abandoned-socket` on `release` (`deregisterCheckout` then dest's body)] → Mitigation: document; do not wait; do not take that checkout table.
- [Freeing the turn before or during the locked section corrupts `inUse = liveCap() - len(inUseTurns)`] → Mitigation: `freeInUseTurn` only after `parkIdleConn` returns; `parkIdleConn` does not free.
- [Double-free if `parkIdleConn` also frees] → Mitigation: one `freeInUseTurn` at the end of `release`; OverFrees tests stay at 0.
- [In-flight `2026-09-13-golangci-lint-harden` names `dial` / `borrow` results in the same file] → Mitigation: do not restyle `dial` / `borrow` here.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy key. Merge after #71.

## Open Questions

None. The panic-inside-lock test question is assumed on `devstate/explore.md`.
