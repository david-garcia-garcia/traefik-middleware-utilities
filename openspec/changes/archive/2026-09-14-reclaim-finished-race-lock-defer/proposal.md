## Why

A nil-Done holder that binds while `Reset` ends the incarnation can start a watcher on a nil `finished` channel, leaking a polling goroutine for the life of the process — the bind-path hole in the PR #77 leak fix. Separately, a panic under `t.mu` is recovered at the Yaegi plugin boundary and wedges every later `Open` on that table.

## What Changes

- Snapshot `slot.finished` under `t.mu` at bind; do not start `watch` when that snapshot is nil.
- Release every lock-held region in `reclaim/table.go` with `defer` (helpers where lock and unlock interleave). Keep hook/`slog` calls and the intentional `close(ready)` sites outside the mutex.
- `Open` on a zero-value `Table{}` (nil `items`) returns an error, matching nil-table / nil-logger. This is an addition, not a substitute for defer.
- Land the two supplied repros; rework the mutex-wedge repro so a panic *inside* a lock-held region still leaves the table usable (`t.Skip` on a non-panicking `Open` is not done).
- Keep `Table.mu`, `slot` fields, and `slotState` names stable for the parallel coverage ticket.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_reclaim_context-lease`: a nil-Done watcher SHALL snapshot `finished` under the table mutex and SHALL NOT start when that snapshot is nil. Every lock-held region SHALL release the mutex with `defer` so a recovered panic cannot leave it held. `Open` SHALL return an error for an uninitialized table (`Table{}` / nil `items`), not panic with the mutex held.

## Impact

- `reclaim/table.go` (dropWhenDone, Open/drop/reclaimLocked helpers, every lock-held region)
- `reclaim/repro_finished_race_test.go` (verbatim from caller)
- `reclaim/repro_mutex_wedge_test.go` (caller file plus lock-held panic injection)
- `openspec/specs/std_go_reclaim_context-lease/spec.md`
- `knowledge/devdocs/std_go_reclaim.md` (bind-path skip; mutex released on panic)
- Parallel ticket `2026-09-14-reclaim-ending-path-test-coverage` adds only `reclaim/table_gaps_test.go` and takes `tab.mu` / `slot` names directly
