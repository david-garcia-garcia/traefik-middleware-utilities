# Requirement
IssueKey: 2026-09-13-reclaim-bug-hook-panic-busy

## Problem
A panic or a nil `create` in code the reclaim table invoked leaves that key mapped `slotBusy` with `ready` never closed. Later `Open` hangs. `Close` of that incarnation never runs. A `Sleep` panic on the production `AfterFunc` path is an unrecovered goroutine panic and crashes the process.

## Current (code)
- `put` registers the key as `slotBusy` with `ready` open, then calls `create()` with no recover. A panic or a nil `create` never sets `createErr`, never unmaps, never `close(ready)`. `reclaim/table.go` `put` (maps in `Open` then `create()`).
- `Open` maps the key `slotBusy` before `put`. Later `Open` waits on `ready`. `reclaim/table.go` `Open` busy wait.
- `drop` sets `slotBusy`, unlocks, `runSleep`, then `slotAsleep` + `close(ready)`. A `Sleep` panic skips unmap, `Close`, and `close(ready)`. `reclaim/table.go` `drop`.
- `dropWhenDone` uses `context.AfterFunc` to call `drop` with no recover. `reclaim/table.go` `dropWhenDone`.
- `reclaimLocked` sets `slotBusy`, unlocks, `runWake`, then `slotAwake` + `close(ready)` + `dropWhenDone`. A `Wake` panic skips those. `reclaim/table.go` `reclaimLocked`.
- `dispose` / `runClose` have no recover. A `Close` panic on an `AfterFunc` goroutine kills the process. `reclaim/table.go` `dispose`, `runClose`.
- `Reset` calls `runSleep` / `dispose` with no recover. `reclaim/table.go` `Reset`.
- `runSleep` / `runWake` / `runClose` do not recover (and must not swallow as success). `reclaim/table.go`.
- Create error (returned, not panic) already sets `createErr`, `slotGone`, unmaps, `close(ready)`. Panic/nil is not that path. `reclaim/table.go` `put` error branch; spec `openspec/specs/std_go_reclaim_context-lease/spec.md`.
- Spec: `wake` cannot fail; no error path; never fallback to `create`. `openspec/specs/std_go_reclaim_value-lifecycle/spec.md`. Same in `knowledge/devdocs/std_go_reclaim.md`.
- Product tests do not cover 2a–2d. The only recover in `reclaim/table_test.go` is `TestTable_OpenNilContextPanics`.

## Desired
The table owns the `slotBusy`/`ready` protocol. A panic (or nil `create`) in code it invoked must not leave that protocol half-finished and must not crash the process.

- `put`: nil `create`, or panic in `create`, is the same as `create` returning an error: set `createErr`, `slotGone`, unmap, `close(ready)`, return that error. No `Close` (nothing stored). Wrap panic as `fmt.Errorf("reclaim: create %q: panic: %v", key, recovered)`.
- `drop` (`Sleep`): recover. Do not park the value asleep for reclaim. End this incarnation: `Close`, unmap, `slotGone`, `close(ready)`. Waiters create a new incarnation.
- `reclaimLocked` (`Wake`): recover. Same end: `Close`, unmap, `slotGone`, `close(ready)`. This `Open` returns an error (wrapped panic), not the pointer. A panicking hook is a broken hook, not a resume failure.
- `dispose`/`Close` panic: recover so `AfterFunc` cannot kill the process. `close(ready)` must already have happened before `Close`.
- Do not recover inside `runSleep`/`runWake`/`runClose` as a silent swallow that continues as if the hook succeeded. Do not re-panic after unsticking. `Reset` uses the same recover.

Tests first, then fix:
1. Land product tests that FAIL on current master (hang / leftover mapped key / `Close` never ran / process panic on AfterFunc `Sleep`). Then implement. Then they PASS. Existing reclaim tests stay green.
2. Example 2a: recover from first `Open` whose `create` panics; assert mapped leftover; second `Open` with timeout must not return until the fix; after fix second `Open` must return and be free to create.
3. Example 2b: `Open(..., create: nil)` panics/nil deref; after recover key still busy; later `Open` hangs. After fix: `Open` returns an error, later `Open` with valid `create` works.
4. Example 2c: `Sleep` panics. Production AfterFunc path must not crash the process. Later `Open` must not hang. `Close` of the broken incarnation should run per agreed how.
5. Example 2d: after `Sleep`, second `Open` `Wake` panics; recover; third `Open` must not hang. After fix: `Open` returns wrapped panic error; later `Open` can create; `Close` ran.

## Affected
- `reclaim/table.go` (`put`, `drop`, `reclaimLocked`, `dispose`, `Reset`)
- `reclaim/table_test.go` (new 2a–2d tests)
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` and `openspec/specs/std_go_reclaim_context-lease/spec.md` (propose: panic/nil-create and panicking-hook end)
- `knowledge/devdocs/std_go_reclaim.md` (`Wake` cannot fail; Sleep identity)

## Out of scope
- Canceled-ctx `Open` returns a closed value (`put` publishes then `AfterFunc` `drop` if `ctx` is already done). `reclaim/table.go` `put` then `dropWhenDone`.
- Unmap-before-`Close` overlap (`expire` unmaps then `dispose`; a new `create` can run while `Close` still runs). `reclaim/table.go` `expire`.
- Silent swallow inside `runSleep`/`runWake`/`runClose`.
- Re-panic after unsticking.

## Unknowns
- Wrapped error format for `Sleep`/`Wake`/`Close` panics (ticket specified the `create` wrap only).
- How the AfterFunc process-crash test is isolated so `go test` of the package does not die with the child.

## Tensions
- Spec and usage packet: `wake` SHALL NOT fail and never falls back to `create`. Ticket: `Wake` panic returns a wrapped error; later `Open` creates a new incarnation. (`openspec/specs/std_go_reclaim_value-lifecycle/spec.md`, `knowledge/devdocs/std_go_reclaim.md`)
- Spec: `Sleep` keeps the value stored for grace. Ticket: `Sleep` panic must not park asleep; end the incarnation (`Close`, unmap, `slotGone`). (`openspec/specs/std_go_reclaim_value-lifecycle/spec.md`)
