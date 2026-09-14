## Context

See proposal.md for why. Dest `reclaim/table.go`: `dropWhenDone` reads `incarnation.finished` at line 414 without `t.mu`; `closeFinished` writes it under the lock. Thirteen `t.mu.Lock()` sites, zero `defer t.mu.Unlock()`. `Open` / `drop` / `reclaimLocked` interleave unlock across branches; `reclaimLocked` is called holding the lock and unlocks itself. `unmapAfterClose` and `endBusyAfterPanic` close `ready` outside the lock on purpose. Proceed policies: `devstate/explore.md`. Spec: `std_go_reclaim_context-lease` (FindSpecHost fold, high). Parallel ticket `2026-09-14-reclaim-ending-path-test-coverage` takes `tab.mu` and `slot` field names in `table_gaps_test.go`.

## Goals / Non-Goals

**Goals:**
- Locked `finished` snapshot plus nil skip so `watch` never receives a nil channel.
- Every lock-held region unlocks via `defer`.
- Wedge repro injects a panic inside a lock-held region (pre-closed `ready` in `put`) and asserts a later `Open`/`Reset` completes.
- Race repro keeps slow-first-`Err` (`slowErrNilDone`); not a slow slog handler.
- Coverage of `./reclaim/` not below DestBranch plus these repros.

**Non-Goals:**
- `reclaim/table_gaps_test.go` (parallel ticket).
- Renaming `Table.mu`, `slot` fields, or `slotState` constants.
- A Yaegi interp panic probe.
- A nil-map guard without the defer refactor.
- Moving hooks, `slog`, or the outside-lock `close(ready)` sites under `t.mu`.

## Decisions

1. **Tiny helper for the finished snapshot.** `dropWhenDone` calls a helper that locks, `defer Unlock`, copies `incarnation.finished`, and returns it. If nil, return without `watch`. Alternative: inline Lock/Unlock in `dropWhenDone` — rejected; defect 2 requires defer, and a two-line lock around one read still needs a helper so defer is legal. Alternative: lock-only without the nil check — rejected; that removes the race detector warning and keeps the leak.

2. **One helper per lock-held region in `Open`, `drop`, and `reclaimLocked`.** Open's loop body becomes a lookup helper that returns a decision (`register` / `bind` / `reclaim` / `wait` / `retryGone`) plus the data the caller needs (incarnation, value, ready, stored hooks). `reclaimLocked`'s "called holding the lock" contract goes away: the asleep case is one lookup result, Wake stays in the caller after unlock. Drop's busy-wait loop returns `ready` to wait on, or the last-holder snapshot. Helpers that must close `ready` outside the lock return that channel. Alternative: `defer Unlock` in the exported methods themselves — rejected; those methods unlock in the middle to run hooks. Alternative: keep `reclaimLocked` taking the lock from the caller — rejected; that cannot `defer` without double-unlock.

3. **Keep `unmapAfterClose` / `endBusyAfterPanic` close-outside-lock.** The helper returns `ready`; the caller `close`s it. Alternative: close under the lock — rejected; DestBranch already proved waiters must not run Close under `t.mu`, and a waiter waking while still inside the closer's lock is the opposite of the existing design.

4. **Wedge repro: pre-close `slot.ready` so `put` panics on `close` inside its locked region.** Keep the zero-value `Open` case as a second path (now an error, not a wedge). Do not treat `t.Skip` when `Open` no longer panics as done. Alternative: only add a nil-map guard — rejected; the Skip would go green and leave every other lock-held panic wedging. Alternative: panic in a hook — rejected; hooks already run outside `t.mu`.

5. **`Open` errors on nil `items`.** Same shape as nil table / nil logger. Taken as an addition. Alternative: leave `Table{}` panicking now that defer would unwedge it — rejected; explore took the error so the exported API cannot panic-under-lock on a zero value.

6. **Stable identifiers.** Do not rename `mu`, `slot`, `slotState`, or slot fields. Alternative: unexport or rename for "clarity" — rejected; parallel white-box tests take those names.

## Risks / Trade-offs

- [Risk] Helper extraction double-closes `ready`. → Mitigation: each busy transition still closes exactly once; helpers return the channel and the caller closes, matching DestBranch's balance.
- [Risk] `defer` in a helper that used to unlock in the middle holds the lock across a hook. → Mitigation: the helper returns before the hook; the caller runs Sleep/Wake/Close/`slog` after unlock.
- [Risk] Nil-map error hides the wedge class from the original repro. → Mitigation: the pre-closed `ready` injection is the acceptance test for defer.
- [Risk] Parallel coverage tests construct `slot` states under `tab.mu`. → Mitigation: names stay; this change does not add `table_gaps_test.go`.
- [Trade-off] More small helpers vs one giant locked method. Helpers are the only way `defer` coexists with unlock-before-hook.

## Migration Plan

Library bugfix. `Open` on `Table{}` changes from panic to error (misuse; production callers use `New`). Rollback is revert. Proof: `go test -count=1 -timeout 10m ./reclaim/` and Docker `golang:1.25 go test -race -count=1 -timeout 10m ./reclaim/`.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
