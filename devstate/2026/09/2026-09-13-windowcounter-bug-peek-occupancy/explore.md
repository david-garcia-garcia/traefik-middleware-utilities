# Explore

## Concepts

- **Occupancy**: Peek's `allowed` is `estimated <= limit` on the already-used sliding estimate. `peekExact` / `peekBuffered` do not add a hit (`windowcounter/limiter.go`).
- **Take**: increment first, then the same compare. After occupancy `limit`, the next Take denies with estimate `limit+1`.
- **Next-hit**: comparing occupancy+1. Ticket forbids changing Peek to that.
- **Allowed-match**: Peek's `allowed` equals the next Take's `allowed` only when that increment does not cross `limit`. Dest `TestPeek_AgreesWithTakeBeforeIncrement` already fills `limit-1`.
- **Miswording**: Peek godoc says “whether a hit would be allowed”. Spec scenario “Take's allowed matches Peek's allowed” has no non-crossing qualifier. Parent repro fails when they differ.

```
  Takes 1..N (N=limit)          Peek                    next Take
  occupancy = N                 allowed true, est=N    increment → N+1
                                (occupancy at limit)    allowed false, est=N+1
```

Measured on dest HEAD (`d1230ec` worktree, throwaway copy of parent `repro_peek_take_boundary_test.go`, then deleted): `limit=3`, exact and buffered: `allowedP=true estP=3 allowedT=false estT=4`.

## Decisions

- Keep Peek compare (`estimated <= limit` without adding one). Occupancy is the contract.
- Tests first: create `windowcounter/repro_peek_take_boundary_test.go` from the parent example; rewrite assertions so the occupancy lock **passes** on dest code.
- Then Peek godoc, `knowledge/devdocs/std_go_windowcounter.md` (How to use + Gotchas), and `openspec/specs/std_go_windowcounter_sliding-take/spec.md` only.
- Do not change `TestPeek_AgreesWithTakeBeforeIncrement` (already the non-crossing case).
- Bound: this occupancy lock and wording only.

## Open questions

- Q: Does the occupancy lock keep the name `TestRepro_PeekAllowsWhenNextTakeDenies`?
  Rank: additive asked — new test file this change creates; Desired line 1 names that rewrite
  Decision: resolved — keep that name; rewrite assertions to occupancy (Peek true/est=N, next Take false/est=N+1).
  By: explore

- Q: What Peek godoc and Gotchas wording?
  Rank: additive asked — Desired line 2 names Peek godoc and usage+Gotchas; no existing callers of the comment string
  Decision: assumed — godoc: Peek reports whether already-used occupancy is at or under limit, not whether the next Take would admit. Gotchas: after occupancy equals limit, Peek allows and the next Take denies and still increments. How to use: Peek is occupancy; do not treat allowed as a reservation for a later Take.
  By: explore

- Q: Does the spec requirement title stay “Peek agrees with Take before the increment”?
  Rank: bounded asked — Desired line 2 names the spec; one requirement + frozen-clock scenario in `openspec/specs/std_go_windowcounter_sliding-take/spec.md`; unit test `TestPeek_AgreesWithTakeBeforeIncrement` already covers non-crossing
  Decision: assumed — keep the title (before-increment is occupancy). Qualify the scenario: allowed-match holds when the increment does not cross limit; at exactly limit Peek may allow and Take deny. Estimated still grows by one hit.
  By: explore
