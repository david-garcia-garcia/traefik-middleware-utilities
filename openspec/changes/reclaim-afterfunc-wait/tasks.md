## 1. Cancellable wait

- [ ] 1.1 Replace the three `go t.watch` sites in `reclaim/table.go` (bind, put, reclaim) with one helper: `Done() != nil` → `context.AfterFunc(ctx, drop)` (do not store the stop func); `Done() == nil` → `go t.watch` / `waitCtx` poll. Do not change `Open`, grace, hooks, logging, or create. Leave `waitCtx` as the poll waiter.

## 2. Compiled and interp proof

- [ ] 2.1 Add compiled hold-time goroutine coverage for cancellable holders (`settledGoroutines`, same slack as `TestTable_GoroutinesReturnToBaseline`). Keep `TestTable_HolderWithoutDoneChannelIsPolled` and `TestTable_GoroutinesReturnToBaseline`.
- [ ] 2.2 Run `go test ./reclaim/...` until passing. Existing `reclaim/yaegi_test.go` GOPATH harness (`stdlib.Symbols`, `useunsafe` false) must still load `table.go`. No new Pester AfterFunc case. Yaegi pin stays v0.16.1.

## 3. Validate

- [ ] 3.1 `openspec validate --change reclaim-afterfunc-wait --strict`
