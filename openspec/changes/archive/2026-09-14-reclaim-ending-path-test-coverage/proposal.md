## Why

On `master`, `reclaim` is 94.4% of statements with nine profile rows at count 0. Two documented Reset contracts (Sleep panic skips orphan and still disposes; Reset unmaps first regardless of `EnforceCloseBeforeOpen`) have no test, so a later ending-path edit can change them while the existing suite still passes.

## What Changes

- Add `reclaim/table_gaps_test.go` verbatim (seven tests). Do not edit `reclaim/table.go`.
- Pin the two Reset contracts with ADDED scenarios on the existing reclaim spec leaves.
- Cover the other seven zero-count blocks from tests (direct `waitCtx`, white-box `Open` slotGone / `drop` busy-wait / `expire` early return). Do not delete those defensive statements.
- Record unreachable/redundant defenses and sibling-lock coupling as `knowledge/debt/` notes (already on the branch).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_reclaim_value-lifecycle`: tests-only `Reset` of an awake value whose Sleep panics SHALL skip orphan, still Close, and still emit `reclaim_dispose`. `Reset` SHALL unmap first regardless of stored `EnforceCloseBeforeOpen`, so a later `Open` creates while that Close is still in flight.
- `std_go_reclaim_context-lease`: `Reset` of an awake value SHALL emit `reclaim_orphan` then `reclaim_dispose` only when Sleep returns. A Sleep panic during `Reset` SHALL NOT emit `reclaim_orphan`.

## Impact

- `reclaim/table_gaps_test.go` (new)
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md`
- `openspec/specs/std_go_reclaim_context-lease/spec.md`
- `knowledge/debt/2026-09-14-reclaim-unreachable-defensive-paths.md`
- `knowledge/debt/2026-09-14-reclaim-whitebox-test-lock-coupling.md`
- No `Open` / `Reset` / `Hooks` signature change. No production behaviour change.
