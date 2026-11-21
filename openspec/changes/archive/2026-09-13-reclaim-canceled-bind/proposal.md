## Why

`Table.Open` can return `(value, nil)` after the holder context is already canceled, so the caller holds a pointer whose Close hook has already run or will run immediately. The reproduced path is cancel during a blocking `create` at zero grace; the same bind-then-`AfterFunc` race exists on awake bind and reclaim.

## What Changes

- **BREAKING** for callers that treated `(value, nil)` as success even when `ctx` was already done: `Open` returns `(nil, ctx.Err())` and does not give the caller the pointer when `ctx.Err() != nil` at bind time.
- After a successful create, awake bind, or reclaim, if `ctx.Err() != nil`, call `drop` on this stack (do not register `AfterFunc`) and return the context error. The value is not leaked. Waiters that bound when `ready` closed still keep the incarnation alive. If `ctx` is still live, keep `AfterFunc`/`watch`.
- Product tests in `reclaim/` that fail on dest (cancel during create, already-done awake bind, already-done reclaim, waiters still get the live value) then pass after the fix.
- Usage packet `knowledge/devdocs/std_go_reclaim.md` documents the canceled-bind return.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_reclaim_context-lease`: `(value, nil)` means this call bound a holder that was still live at return. A canceled ctx at bind returns `(nil, ctx.Err())` and does not receive the pointer. Concurrent first Opens still create once; waiters that bound when ready closed still receive that live value.

## Impact

- `reclaim/table.go` (`put`, `Open` awake bind, `reclaimLocked`, `dropWhenDone`)
- `reclaim/table_canceled_bind_test.go` (new)
- `openspec/specs/std_go_reclaim_context-lease/spec.md`
- `knowledge/devdocs/std_go_reclaim.md`
- Callers of `Open` that cancel during create or pass an already-done ctx must handle the context error instead of a pointer
