# Explore
IssueKey: 2026-09-13-reclaim-bug-canceled-open-close

## Concepts

```
  Open (create)                         dest today
  --------------                        ----------
  create()  ──ctx canceled mid-create──►
  put: holders++, slotAwake, close(ready)
  dropWhenDone → AfterFunc(ctx, drop)   AfterFunc fires in its own goroutine
  return (value, nil)                   caller holds the pointer
       │                                drop: last holder, zero grace → Sleep+Close
       └── caller uses value ──────────► Close may already have run
```

Agreed how: `(value, nil)` means this call bound a holder that was still live at return. If `ctx.Err() != nil` at bind time, Open returns `(nil, ctx.Err())` and does not give the caller the pointer. After a successful create, awake bind, or reclaim: if `ctx.Err() != nil`, call `drop` on this stack (do not register AfterFunc) and return the context error. If ctx is still live, keep AfterFunc/watch.

Three sites, same steps: `put`, awake bind in `Open`, `reclaimLocked`.

Waiters that bound when `ready` closed still keep the incarnation alive: creator `drop` decrements one holder; it does not unmap while `holders > 0`.

## Current (measured)

- `reclaim/table.go` `put` (after `create`), `Open` `slotAwake`, and `reclaimLocked` all call `dropWhenDone` then return the pointer with `nil` error. None check `ctx.Err()` at bind.
- `dropWhenDone` always registers `context.AfterFunc` when `ctx.Done() != nil`. Go runs `f` in its own goroutine even when ctx is already done, so Close is racing the return, not serialized on it.
- Throwaway of the ticket example (50 inner loops, `-count=20` = 1000 iters) **did not** fail the “Close already ran before Open returned” assertion on this runner: AfterFunc had not finished Close at the return instruction.
- A follow-up throwaway that waited up to 2s after `(value, nil)` **did** fail on iter 0: dest returned `ptr` with `err=nil`, then Close ran. The doomed pointer is reproduced; the pre-return Close check is racy.
- `TestTable_ZeroGraceOpenRacesCancel` covers a second **live** Open racing the first cancel; it does not cover cancel during blocking create, nor an already-done ctx on bind/reclaim.
- Spec `std_go_reclaim_context-lease` still says concurrent first Opens “every caller receives that one value” with no canceled-ctx exception. Ticket wins; propose updates the spec.
- Usage `knowledge/devdocs/std_go_reclaim.md` does not mention canceled bind. Do not rewrite it in explore (would describe behavior dest does not have yet). Propose + devdocsimpact update it with the change.

## Decisions

- Implement the agreed how at the three sites. Do not only delay AfterFunc. A pre-create `ctx.Err()` check is extra, not the fix (cancel during create is the reproduced path).
- Tests first in `reclaim/`, asserting the **agreed return contract** so `go test ./reclaim` fails on dest (`(value, nil)` today) and passes after the fix. Do not keep the example’s `if err != nil { Fatal }` as the reproduce assertion — dest returns nil error, and Close-before-return is racy on this runner.
- Do not fix hook-panic bricks key or unmap-before-Close overlap.
- No third-party research: this is in-tree `reclaim` plus stdlib `context.AfterFunc`.
- Identity: not in play (no client address / tenant / Host reconstruction).

## Open questions

- Q: Where do the new product tests live?
  Rank: additive asked — new test file this change creates; criterion 1 names package `reclaim` tests
  Decision: assumed — `reclaim/table_canceled_bind_test.go` so the fail-then-green contract is one file next to `table.go`, not mixed into the 1600-line `table_test.go`.
  By: explore

- Q: Do awake-bind and reclaimLocked tests use only `NewTable(0)`?
  Rank: additive asked — test setup for criteria 3; existing ZeroGrace tests already use 0
  Decision: assumed — create-cancel and awake bind use `NewTable(0)` (Close may run because the last holder is done). `reclaimLocked` uses a short positive grace, wait for orphan/asleep, then Open with already-done ctx — zero grace has no sleep window, so reclaim is not an observable site there.
  By: explore

- Q: Does `reclaimLocked` grow an error return or does Open check after it?
  Rank: bounded asked — 1 unexported caller enumerated (`Open` `slotAsleep` in `reclaim/table.go`); criterion 6 names the site
  Decision: assumed — `reclaimLocked` returns `(any, error)` so the three sites share the same bind-then-maybe-drop shape. `Open` `slotAsleep` becomes `return t.reclaimLocked(...)`. Do not keep `return t.reclaimLocked(...), nil`.
  By: explore

- Q: Does implement also add the optional pre-create `ctx.Err()` check?
  Rank: additive asked — ticket: that check may be added as an extra, not instead
  Decision: assumed — do not abort `put` before `create`. That would leave `ready` waiters parked (deadlock) or replay `createErr` to a live waiter. The extra does not replace bind-time drop after create, awake bind, and reclaim. A canceled ctx on a sleeping key still reclaims (wake) then drops, so Open-entry return is not used either.
  By: implement

- Q: How do we cover waiters that still get the live value without a flake?
  Rank: additive asked — criterion 3 names waiters that bound when ready closed
  Decision: assumed — two first Opens, create blocks until the second waits on `ready`, then cancel the creator. Waiter’s table uses positive grace so a lost awake-bind race still reclaims the same pointer instead of creating at zero grace.
  By: implement
