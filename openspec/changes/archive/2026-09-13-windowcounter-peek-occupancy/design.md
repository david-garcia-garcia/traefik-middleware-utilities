## Context

Dest Peek (`peekExact` / `peekBuffered`) already compares `estimated <= limit` without adding a hit. Take increments first, then the same compare. Parent `repro_peek_take_boundary_test.go` asserts Peek allowed equals the next Take's allowed and fails on dest. See proposal.md for why. Spec fold: `std_go_windowcounter_sliding-take`. Explore: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Occupancy lock that passes on dest Peek (exact and buffered).
- Godoc, usage, and the sliding-take requirement state occupancy, not next-hit.

**Non-Goals:**
- Changing Peek compare to occupancy+1.
- Rewriting Take increment-then-compare.
- Other windowcounter bugs.
- Changing `TestPeek_AgreesWithTakeBeforeIncrement` (already fills `limit-1`).

## Decisions

1. **Tests first from the parent example.** Create `windowcounter/repro_peek_take_boundary_test.go` with the same fill (N Takes at frozen clock, exact + buffered). Rewrite assertions to occupancy: Peek allowed true / est=N; next Take allowed false / est=N+1. Keep `TestRepro_PeekAllowsWhenNextTakeDenies`. Alternative: a red-fail that Peek allowed must match Take — rejected; occupancy is the contract.

2. **Do not change Peek compare.** `estimated <= float64(limit)` without adding one stays. Alternative: next-hit (`occupancy+1`) — rejected; ticket forbids it.

3. **Fold into `std_go_windowcounter_sliding-take`.** Small adjustment to “Peek agrees with Take before the increment.” Keep the title. Qualify allowed-match; add occupancy-at-limit scenario. Do not add a sync-flush delta: occupancy is the same compare in both modes; the lock test covers both. Alternative: new leaf — rejected; this is one requirement on the existing Take/Peek contract.

4. **Godoc and usage from explore assumed wording.** Peek godoc: already-used occupancy at or under limit, not whether the next Take would admit. How to use: do not treat Peek allowed as a reservation for a later Take. Gotchas: after occupancy equals limit, Peek allows and the next Take denies and still increments. Keep the Peek-then-Take snippet; the Gotcha is the occupancy split.

## Risks / Trade-offs

- [Callers already treat Peek allowed as a reservation at occupancy=limit] → Mitigation: document the split; do not change Peek. Those callers already race Take deny today.
- [Keeping the repro test name looks like a fail-lock] → Mitigation: comments and assertions state occupancy; the test must pass.

## Migration Plan

Docs and a passing test. Rollback is revert. Peek compare does not move.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
