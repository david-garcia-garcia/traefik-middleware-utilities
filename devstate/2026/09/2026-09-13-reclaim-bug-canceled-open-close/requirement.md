# Requirement
IssueKey: 2026-09-13-reclaim-bug-canceled-open-close

## Problem
Zero-grace `Table.Open` can return `(value, nil)` after that call’s holder context is already canceled, so the caller holds a pointer whose Close hook has already run. The reproduced path is cancel during a blocking `create`. The same bind-then-`AfterFunc` race exists on awake bind and reclaim.

## Current (code)
- `reclaim/table.go` `Open` `slotAwake` — increments `holders`, unlocks, `dropWhenDone`, returns `(value, nil)`. No `ctx.Err()` check at bind.
- `reclaim/table.go` `put` — runs `create()` with no `ctx.Err()` check after it; on success sets `slotAwake`, `holders++`, closes `ready`, then `dropWhenDone` and returns `(value, nil)`.
- `reclaim/table.go` `reclaimLocked` — `holders++`, `runWake`, then `dropWhenDone`, returns `any` only. `Open` `slotAsleep` is `return t.reclaimLocked(...), nil`.
- `reclaim/table.go` `dropWhenDone` — if `ctx.Done() != nil`, `context.AfterFunc(ctx, drop)` (fires immediately when `ctx` is already done); else `watch`.
- `reclaim/table.go` `drop` / `expire` — last holder at zero grace unmaps, `runSleep` then Close; Close can finish before `Open` returns the pointer.
- `reclaim/table_test.go` `TestTable_ZeroGraceEndsImmediately` — cancels **after** `Open` returns; does not cover cancel during create.
- `reclaim/table_test.go` `TestTable_ZeroGraceOpenRacesCancel` — second **live** `Open` racing first cancel; asserts that second caller’s pointer is not already closed.
- Dest `reclaim/` has no test that cancels during blocking `create` and asserts Close already ran while `Open` returned the pointer.
- `openspec/specs/std_go_reclaim_context-lease/spec.md` — concurrent first `Open`s: every caller receives that one value; a later `Open` (live or sleeping) SHALL return the stored value and bind the new context. No exception for `ctx.Err() != nil` at bind.
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` — `Open` SHALL NOT return a stored value before `wake` has returned; does not say a canceled ctx must not receive the pointer.
- `knowledge/devdocs/std_go_reclaim.md` — second `Open` while live or in grace returns the same value.

## Desired
1. Land product tests in `reclaim/` that **fail on current dest** (`go test ./reclaim`). Do not weaken them to pass. Tests first, then the fix.
2. Reproduce cancel during blocking `create` at zero grace: fail while Close already ran and `Open` returned the pointer (adapt the ticket example). After the fix, that test’s contract is: `Open` returns `ctx.Err()` and **not** the pointer; Close **may** run because the holder was already done.
3. Cover awake bind and `reclaimLocked` with already-done `ctx` (same contract). Cover that waiters that bound when `ready` closed still get the live value.
4. Agreed how (not a different design): `(value, nil)` means this call bound a holder that was still live at return. If `ctx.Err() != nil` at bind time, `Open` returns `(nil, ctx.Err())` and does not give the caller the pointer.
5. After a successful create, awake bind, or reclaim: if `ctx.Err() != nil`, call `drop` on this stack (do not register `AfterFunc`) and return the context error. The value is not leaked: `drop` still sleeps/graces/closes it; waiters that bound when `ready` closed still keep it alive. If `ctx` is still live, keep `AfterFunc`/`watch`.
6. Same three sites: `put`, awake bind in `Open`, `reclaimLocked`.
7. Do not only delay `AfterFunc` until after return. A pre-create `ctx.Err()` check may be added as an extra, not instead: the reproduced case cancels during create.
8. Then those tests PASS and existing `reclaim` tests stay green.

## Affected
- `reclaim/table.go` (`put`, `Open` awake bind, `reclaimLocked`, `dropWhenDone`)
- `reclaim/` tests (new failing-then-green coverage; likely `table_test.go`)
- `openspec/specs/std_go_reclaim_context-lease/spec.md` (bind contract vs canceled ctx)
- `knowledge/devdocs/std_go_reclaim.md` (Open return when holder ctx is already done)

## Out of scope
- Hook panic bricks the key
- Unmap-before-Close overlap
- Delaying `AfterFunc` until after return as the only change
- Treating a pre-create `ctx.Err()` check as the only fix
- Other packages (`simpleredis`, `tokenbucket`, `windowcounter`)

## Unknowns
- Whether new tests land in `reclaim/table_test.go` or a new `reclaim/` test file (ticket says package `reclaim` tests; example names one function).
- Whether awake-bind / reclaim already-done tests use only `NewTable(0)` or also a positive grace.
- Whether `reclaimLocked` grows an error return or `Open` checks `ctx` after it (ticket names the site, not the helper signature).
- Whether implement also adds the optional pre-create `ctx.Err()` check.

## Tensions
- Ticket: canceled bind returns `(nil, ctx.Err())`. Spec `std_go_reclaim_context-lease` “every caller receives that one value” and “a later Open SHALL return the stored value” have no canceled-ctx exception. Ticket wins; spec/usage must catch up in propose.
- Ticket: do not only delay `AfterFunc` until after return. Dest currently always registers `AfterFunc` before return; delaying it would still give the caller a pointer whose Close can already have run.
- Example reproducer fatals on `err != nil`; after the fix that same test must expect `ctx.Err()` and a nil pointer. Do not keep the pre-fix assertion.
- Waiters that bound when `ready` closed still receive the live value; the canceled call that created or rebound does not.
