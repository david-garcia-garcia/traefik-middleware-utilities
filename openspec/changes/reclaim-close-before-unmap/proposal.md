## Why

Zero grace (and expire after a positive grace) unmaps the key, then runs Close outside `t.mu`. A concurrent `Open` sees the key absent and `create`s the next incarnation while the previous Close is still in flight. Traefik reload is that sequence. Close of an incarnation must finish before the key is absent for a new create.

## What Changes

- Land a compiled product test that fails on dest when create of incarnation 2 starts while Close of 1 is blocked (`TestTable_ZeroGraceCreateWaitsUntilPreviousCloseReturns`). Cover expire after a positive grace the same way (`TestTable_ExpireCreateWaitsUntilPreviousCloseReturns`).
- Keep the key mapped `slotBusy` for the whole Close, then unmap and `close(ready)`. `Open` already waits on `slotBusy`; after `ready` it sees the key gone and creates.
- Zero grace: after Sleep, do not publish `slotAsleep` and do not unmap. Close as part of that same busy transition, then unmap / `close(ready)`.
- expire: do not delete until Close returns. Switch to `slotBusy` (not asleep) for the Close window so a racing `Open` cannot reclaim.
- Do not run Close under `t.mu`. Do not leave the slot asleep during Close.
- Do not change tests-only `Reset` unmap-first (existing Reset tests depend on it; production never calls `Reset`).
- Do not fix canceled-ctx disposed return or hook-panic bricks the key.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_reclaim_value-lifecycle`: `Open` waits while Close is in flight for that key, the same way it waits for an in-flight Open, sleep, or create. The key is unmapped only after Close returns.
- `std_go_reclaim_context-lease`: The key stays mapped until Close returns. Zero grace still has no sleeping window. An `Open` that races a zero-grace (or expire) Close waits, then creates; it MUST NOT reclaim and MUST NOT start `create` until that Close has returned.

## Impact

- `reclaim/table.go` (`drop` zero-grace path, `expire`, slot state during Close, comments that already claim Close runs as `slotBusy`).
- `reclaim/table_test.go`: new overlap tests first (fail on dest), then pass after the table change. Existing reclaim tests stay green.
- Live specs `std_go_reclaim_value-lifecycle` and `std_go_reclaim_context-lease`.
- Usage packet `knowledge/devdocs/std_go_reclaim.md` after apply (Open waits on Close for that key).
- No `Open` signature change. No `Reset` reorder. No other reclaim bugs.
