# Nitpicks

1. [hard] Name for the scope — `simpleredis/resp.go:26` — `ioBound` still reads like the removed per-operation `IOTimeout` cap, but the value is `commandBudgetLeft(ctx)` (remaining whole-command budget for one `SetDeadline` stamp); the block comment above it rejects a second per-operation bound, so the local name makes the reader decode the old model.
   → Rename to the role in this body, e.g. `budgetLeft := sr.commandBudgetLeft(ctx)` and use that name through the `SetDeadline` guard.
   Status: done
   Argument: renamed the local to `budgetLeft` through the `SetDeadline` guard in `do`; `simpleredis/resp.go`.
