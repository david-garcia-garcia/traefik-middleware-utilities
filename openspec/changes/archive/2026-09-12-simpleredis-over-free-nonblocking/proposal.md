## Why

On DestBranch, returning an in-use-turn token when the semaphore is already full parks the goroutine forever: no error, no log, no panic. Dest borrow/release pairing is balanced today; this is how a future double-free (a cancelled command that returns early and is later released) would hang Traefik requests until the process runs out of memory.

## What Changes

- Returning an in-use turn when the channel is already full MUST NOT block. Drop that send and increment `OverFrees()`.
- Export `OverFrees() int64` next to `PoolSize()` / `MaxIdleConns()` so a compiled test can read the count. Correct accounting stays at 0.
- Guard test: an extra return on a fresh client returns promptly and `OverFrees()` is 1.
- Invariant test: hammer borrow/release exits (healthy fake, dead address, AUTH reject, starved pool with a short `PoolTimeout`) from many goroutines; assert the turn channel is full and `OverFrees() == 0`.
- No panic. No mutex-counter rewrite. No Traefik probe wiring. No CI `-race` workflow change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: an extra return of an in-use turn MUST NOT hang; `OverFrees()` increments on that drop and stays 0 on balanced accounting.

## Impact

- `simpleredis/pool.go` (`freeInUseTurn` send).
- `simpleredis/simpleredis.go` (`overFrees` field, `OverFrees()`).
- `simpleredis/pool_test.go` (guard + invariant).
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` (over-free hang gotcha).
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Sibling findings in `simpleredisfixes2/` stay other tickets.
