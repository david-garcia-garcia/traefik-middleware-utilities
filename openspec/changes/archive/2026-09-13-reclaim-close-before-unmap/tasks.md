## 1. Failing overlap tests (dest)

- [x] 1.1 Add `TestTable_ZeroGraceCreateWaitsUntilPreviousCloseReturns` in `reclaim/table_test.go`: `NewTable(0)`, Close hook signals then blocks, cancel last holder, concurrent `Open` of the same key. `t.Fatal` if create of incarnation 2 runs while Close of 1 is blocked (200ms wait). Then release Close and require the second Open to create after Close returns.
- [x] 1.2 Add `TestTable_ExpireCreateWaitsUntilPreviousCloseReturns` for expire after a positive grace: same blocked-Close overlap, `t.Fatal` if create starts during Close. Wait for Close to enter (grace elapsed) before the second Open.
- [x] 1.3 Run `go test ./reclaim/ -count=1 -run 'TestTable_ZeroGraceCreateWaitsUntilPreviousCloseReturns|TestTable_ExpireCreateWaitsUntilPreviousCloseReturns'` and record that both FAIL on dest (overlap). Do not change `table.go` yet.

## 2. Close-before-unmap

- [x] 2.1 In `drop`, after Sleep at zero grace: do not publish `slotAsleep`, do not unmap, do not `close(ready)`. Close as part of that same `slotBusy` transition, then unmap / `close(ready)`, then the dispose log. Do not run Close under `t.mu`.
- [x] 2.2 In `expire`: if still asleep and holders are 0, switch to `slotBusy` (new `ready`) under `t.mu`, unlock, `runClose`, then unmap and `close(ready)`, then the dispose log. Do not delete until Close returns. Do not leave the slot asleep during Close. Do not call `dispose` in a way that Closes twice.
- [x] 2.3 Leave tests-only `Reset` unmap-first. Do not fix canceled-ctx disposed return or hook-panic bricks the key. Update package / `slotBusy` comments so they match Close-as-busy.

## 3. Proof and usage

- [x] 3.1 Re-run the two overlap tests until they PASS. Run `go test ./reclaim/...` until existing reclaim tests stay green.
- [x] 3.2 Update `knowledge/devdocs/std_go_reclaim.md` so Language Close / Gotchas say `Open` waits until Close returns for that key, then creates. Append `devstate/knowledge.md`.
- [x] 3.3 `openspec validate --change reclaim-close-before-unmap --strict`
