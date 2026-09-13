## Why

A panic or a nil `create` in code the reclaim table invoked leaves that key mapped `slotBusy` with `ready` never closed. Later `Open` hangs. Close never runs. A Sleep panic on the production `AfterFunc` path is an unrecovered goroutine panic and kills the process.

## What Changes

- The table recovers panics (and a nil `create`) at the protocol owners (`put`, `drop`, `reclaimLocked`, `dispose`, `Reset`) so the slot is never left half-finished and AfterFunc cannot crash the process.
- Nil `create` or panic in `create` is the same as `create` returning an error: `createErr`, `slotGone`, unmap, `close(ready)`, return that error. No Close.
- Sleep panic: do not park the value asleep. End this incarnation: Close, unmap, `slotGone`, `close(ready)`. Waiters create a new incarnation.
- Wake panic: same end. This `Open` returns a wrapped panic error, not the pointer. Concurrent waiters replay `createErr`. A later `Open` creates.
- Close panic: recover so AfterFunc cannot kill the process. `close(ready)` already happened.
- Product tests for 2a–2d land first (fail on current master), then the recover, then they pass.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_reclaim_context-lease`: nil `create` and panic in `create` SHALL follow the existing create-error path (key not stored, waiters receive the error, later `Open` may create).
- `std_go_reclaim_value-lifecycle`: a panicking Sleep or Wake SHALL end the incarnation (Close, unmap) instead of parking asleep or returning the pointer; Wake panic SHALL return an error from `Open`; Close panic SHALL NOT crash the process.

## Impact

- `reclaim/table.go` (`put`, `drop`, `reclaimLocked`, `dispose`, `Reset`). Unexported `reclaimLocked` returns `(any, error)`. Public `Open` already returns `(any, error)`.
- `reclaim/table_test.go` (2a–2d). AfterFunc crash proof is a subprocess.
- `knowledge/devdocs/std_go_reclaim.md` (Wake cannot fail; Sleep identity on panic).
- Production caller `e2e/reclaimprobe/plugin.go` already checks `Open` error. No new dependencies.
- Out of scope: canceled-ctx Open returns a closed value; unmap-before-Close overlap.
