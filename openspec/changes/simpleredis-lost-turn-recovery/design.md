## Context

Dest `inUseTurns` is filled once in `New`. `exec` calls `release` without `defer` (PR 29 discarded deferred release). `OverFrees()` counts extra returns only. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Refill leaked turns at pool-wait when owned sockets are zero.
- Distinguish that case from a saturated busy pool.
- `LostTurns()` read-only next to `OverFrees()`.
- Default-suite tests: recovered panic after `borrow` succeeds; busy pool still caps; `LostTurns()` reports the leak.

**Non-Goals:**
- `defer sr.release` in `exec`.
- Goroutine reaper / leased-turn object.
- Closing TCP fds left behind after panic.
- `handshakeFailure` / `shouldRetry` rewrite; `resp.go`; `BUGS.md`.

## Decisions

1. **Refill at `errPoolWait`, not defer-release.** PR 29 owner discarded defer-release as too much complexity for Yaegi recovering the request while the turn stays lost. Alternative: lease + reaper — `New` MUST NOT start a goroutine.

2. **`heldSockets` atomic, not increment-at-turn-take.** Increment in `exec` after a successful `borrow` with `defer` decrement (Traefik unwind restores it). Increment around `dial` inside `borrow` so a waiter during `DialContext` does not refill. Owned live = `len(idleConns)` + `heldSockets`. Alternative: increment at turn-take and decrement only in `release` — leaks the same way as the turn, so `bugPanicAfterBorrow` would look busy forever.

3. **Refill under `idleConnsMu` (or a dedicated turn mutex) with re-check.** Multiple waiters can time out together. Restore `cap-len` tokens once, add that count to `lostTurns`, then take a turn and proceed.

4. **Tests in `simpleredis/pool_test.go` (or a sibling untagged `_test.go`).** Port `bugPanicAfterBorrow` untagged. Busy-pool case can share the `getDelay` shape of `TestPoolWaitTimesOutWithoutExtraDial` and assert `LostTurns()==0`. Do not use `//go:build bugrepro`.

## Risks / Trade-offs

- [Recovery fires while sockets are busy and dials past `PoolSize`] → Mitigation: `heldSockets` around `do` and `dial`; busy-pool test.
- [Two waiters double-refill] → Mitigation: lock + re-check `len(inUseTurns)` and owned sockets.
- [Leaked TCP fds after panic] → Accepted; explore assumed not taken.
- [Yaegi `atomic.Int64`] → Mitigation: dest already uses `atomic.Int64` for `overFrees`.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
