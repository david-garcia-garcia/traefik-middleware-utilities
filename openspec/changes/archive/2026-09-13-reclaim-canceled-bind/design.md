## Context

See proposal.md — Why. Dest `reclaim/table.go` `put`, `Open` `slotAwake`, and `reclaimLocked` increment holders, then `dropWhenDone` (`context.AfterFunc` when `Done` is non-nil), then return `(value, nil)`. Go runs AfterFunc in another goroutine even when ctx is already done, so Close can finish before or just after return while the caller holds the pointer.

Explore decisions: tests in `reclaim/table_canceled_bind_test.go`; `reclaimLocked` returns `(any, error)`; extra pre-create `ctx.Err()` check in `put`; waiter test with create blocked until the second Open waits on `ready`.

## Goals / Non-Goals

**Goals:**
- Same three bind sites refuse to return a pointer when `ctx.Err() != nil`.
- Drop the canceled holder on this stack so the value is not leaked.
- Tests fail on dest, then pass after the fix.

**Non-Goals:**
- Delay AfterFunc until after return as the only change.
- Treat a pre-create `ctx.Err()` check as the only fix.
- Hook panic bricks the key; unmap-before-Close overlap.

## Decisions

### Bind-time `ctx.Err()` then drop on this stack
After holders++ and publishing (create: close `ready`; reclaim: wake then mark awake), if `ctx.Err() != nil`, call `drop` on this stack and return `(nil, ctx.Err())`. Do not register AfterFunc for that call. If ctx is live, keep AfterFunc/watch.

Alternative considered: delay AfterFunc until after return. Rejected: the caller still receives a pointer whose Close may already have run.

Alternative considered: only check `ctx.Err()` before `create`. Rejected: the reproduced path cancels during create. That check is extra at the start of `put`, not instead.

### `reclaimLocked` returns `(any, error)`
One unexported caller (`Open` `slotAsleep`). Symmetric with `put`. `Open` `slotAsleep` is `return t.reclaimLocked(...)`.

### Tests first
Land `reclaim/table_canceled_bind_test.go` asserting the agreed return contract so `go test ./reclaim` fails on dest. Then apply the three-site fix. Do not weaken assertions. Cover: cancel during create at zero grace; already-done awake bind at zero grace; already-done reclaim at short positive grace; waiters still get the live value.

## Risks / Trade-offs

- [Risk] AfterFunc vs return is racy, so a “Close already ran before return” assertion may pass on dest. → Mitigation: tests assert `(nil, ctx.Err())` and a nil pointer (fails on dest because Open returns the pointer). Close MAY run after the fix.
- [Risk] Creator `drop` races a waiter bind. → Mitigation: publish (awake, holders++, close ready) before the canceled creator drops; waiter test keeps the waiter’s ctx live.
- [Trade-off] Canceled reclaim still runs Wake then immediately Sleep if that was the last holder. Required so no caller receives a sleeping value.

## Migration Plan

Library change. Callers that canceled during Open and ignored the possibility of a context error must handle `(nil, ctx.Err())`. Rollback is revert of this change. No stored data.
