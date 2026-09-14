# Requirement
IssueKey: 2026-09-14-reclaim-bug-finished-race-lock-defer

## Problem
Two concurrency defects in `reclaim/table.go` on `master`: (1) `dropWhenDone` reads `incarnation.finished` without `t.mu` while `closeFinished` mutates it under the lock, reviving the nil-Done watcher leak on the bind path; (2) no lock-held region uses `defer` to unlock, so a panic under Yaegi recovery can leave `t.mu` held and wedge all future `Open` calls.

## Current (code)
- `reclaim/table.go:409-415` — `dropWhenDone` reads `incarnation.finished` and starts `watch` without synchronizing with `closeFinished` (`reclaim/table.go:187-192`).
- `reclaim/table.go:421-425` — `watch` with nil `finished` polls via `waitCtx` when `ctx.Done()` is nil (`reclaim/repro_nildone_watcher_test.go` on branch documents the prior leak class).
- `reclaim/table.go` — thirteen `t.mu.Lock()` sites, zero `defer t.mu.Unlock()`; interleaved unlock across branches in `Open`, `drop`, `reclaimLocked`.
- `reclaim/table.go:288-295` — zero-value `Table{}`: `Open` locks then assigns `t.items[key]` on nil map (panic with mutex held); nil `*Table` returns error at `reclaim/table.go:277-278`.
- `reclaim/repro_finished_race_test.go` — not on branch (main checkout only): `-race` read/write pair and goroutine leak repro.
- `reclaim/repro_mutex_wedge_test.go` — not on branch (main checkout only): post-panic table wedge repro.

## Desired
- Synchronize `finished` read under lock in `dropWhenDone`, skip starting `watch` when `finished` is nil after bind (caller-verified fix shape); use defer-based unlock helper(s), not bare Lock/Unlock for that read.
- Refactor every `table.go` mutex region to release via `defer` (helpers as needed); preserve channel closes outside lock for `unmapAfterClose` / `endBusyAfterPanic`; keep hook and `slog` calls outside `t.mu`; keep `closeFinished` idempotent and single `close(ready)` per busy transition.
- Rework defect-2 repro to inject panic inside a lock-held region (e.g. double-close `ready` in `put`) and assert table still usable within a time budget; keep race repro mechanism (slow first `ctx.Err()`, not slow slog).
- Optionally return error for zero-value `Table{}` on `Open` (addition only, not substitute for defer refactor).
- Keep `Table.mu`, `slot` fields, and `slotState` names stable for parallel ticket `2026-09-14-reclaim-ending-path-test-coverage`.

## Affected
- `reclaim/table.go` (primary)
- `reclaim/repro_finished_race_test.go`, `reclaim/repro_mutex_wedge_test.go` (implement phase, from main reference)
- CI: unit tests and Docker `-race` on `./reclaim/`

## Out of scope
- `reclaim/table_gaps_test.go` (parallel ticket)
- Copying or committing repro tests during prepare
- Nil-map-only guard without defer refactor
- Widening race repro with slow slog handler

## Unknowns
- Exact helper decomposition for `Open`/`drop`/`reclaimLocked` while preserving intentional unlock-before-hook behavior in `reclaimLocked`.

## Tensions
- None between ticket and measured code on `master`.
