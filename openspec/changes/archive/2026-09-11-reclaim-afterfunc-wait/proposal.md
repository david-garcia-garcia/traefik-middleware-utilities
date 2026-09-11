## Why

Every successful `Open` parks one goroutine for a cancellable holder until that context is done. Traefik's real path is cheap (one Open per plugin instance per reload), but many Opens would pay a parked waiter each. Yaegi v0.16.1 can call `context.AfterFunc`, so the table can drop that parked waiter on `Done() != nil` holders without changing `Open`.

## What Changes

- Bind a cancellable holder (`Done() != nil`) with `context.AfterFunc` so the table does not park a waiter for the hold. Register `drop`; do not store AfterFunc's stop func.
- Keep today's `go watch` / `waitCtx` poll when `Done()` is nil (`nilDoneCtx`, `context.Background()`). AfterFunc never runs `f` on those holders.
- Prove the hold-time goroutine drop in compiled `reclaim` tests. Existing `reclaim/yaegi_test.go` GOPATH harness (`useunsafe` false) must still load `table.go`. No new Pester AfterFunc case. Yaegi pin stays v0.16.1.
- Do not change `Open`, grace, hooks, logging, or the create signature.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_reclaim_context-lease`: Add scenarios that a cancellable holder does not park a waiter for the hold, a nil-`Done` holder stays live until `ctx.Err()` is set, and table goroutines still return to baseline once holders are Done and the incarnation has ended. No wait-mechanism SHALL. Do not rewrite `std_go_reclaim_value-lifecycle`.

## Impact

- `reclaim/table.go` (`waitCtx` / `watch` / the three `go t.watch` sites at bind, put, and reclaim). `reclaim/default.go` is unchanged (`Open` still forwards).
- `reclaim/table_test.go`: keep `TestTable_HolderWithoutDoneChannelIsPolled` and `TestTable_GoroutinesReturnToBaseline`; add compiled hold-time coverage for cancellable holders.
- `reclaim/yaegi_test.go` copies non-test sources into GOPATH; it must still load `table.go` after the AfterFunc call lands.
- No Pester, probe, grace, hooks, logging, or Yaegi pin change. Usage packet `knowledge/devdocs/std_go_reclaim.md` still describes how to call `Open`; AfterFunc is an internal wait.
