## 1. Tests that fail on dest

- [x] 1.1 Add `reclaim/table_canceled_bind_test.go` with cancel-during-create at `NewTable(0)`: `Open` MUST return `ctx.Err()` and a nil pointer; Close MAY run
- [x] 1.2 Cover already-done ctx on an awake bind at zero grace (same return contract); a live sibling holder MUST keep the value
- [x] 1.3 Cover already-done ctx on reclaim at short positive grace after orphan (same return contract; wake has returned)
- [x] 1.4 Cover two first Opens: create blocks until the waiter is on `ready`, then cancel the creator; waiter MUST get the live pointer; creator MUST get `ctx.Err()`
- [x] 1.5 Run `go test ./reclaim` and confirm the new tests FAIL on dest (do not weaken them)

## 2. Bind-time drop

- [x] 2.1 In `put`, after a successful create is published, if `ctx.Err() != nil` call `drop` on this stack (no AfterFunc) and return `(nil, ctx.Err())`; if live, keep AfterFunc/watch
- [x] 2.2 Do not abort `put` before `create` (that would deadlock or starve waiters); bind-time drop after create remains the fix
- [x] 2.3 In `Open` awake bind, after holders++, if `ctx.Err() != nil` call `drop` on this stack (no AfterFunc) and return `(nil, ctx.Err())`
- [x] 2.4 Change `reclaimLocked` to return `(any, error)` and apply the same bind-time drop; `Open` `slotAsleep` returns that pair

## 3. Confirm and usage

- [x] 3.1 Run `go test ./reclaim` and confirm the new tests PASS and existing reclaim tests stay green
- [x] 3.2 Update `knowledge/devdocs/std_go_reclaim.md` Open/gotchas: canceled ctx at bind returns the context error, not the pointer
