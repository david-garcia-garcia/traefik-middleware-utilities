## Why

`stopFlushAndWait` uses one `stopping` bool. Sleep #1 sets it, detaches the timer, and waits on `flushBusy` for the in-flight tick. A concurrent Sleep #2 finds `flushTimer == nil`, sets `stopping=false`, and returns at once. Reproduced black-box on dest: with a flush EVAL held, Sleep #2 returns before the flush ends (its caller believes pending hits are flushed), and a Wake in that gap starts a second timer chain; Sleep #1's `flushBusy.Wait()` then waits for a chain that re-arms forever and never returns. Through reclaim this is unlikely (`claimDrop` serializes the last-holder Sleep), but several owners sharing one `Limiter` hit it.

## What Changes

- `stopping bool` becomes a stop counter (`stoppers int`). Every `stopFlushAndWait` increments it, detaches the timer if there is one, waits on `flushBusy`, then decrements. A caller that finds no timer still waits; it never clears another caller's stop.
- `Wake` and `flushAfterTick` refuse to arm while `stoppers > 0` (same checks as today on the bool).
- The `flushBusy.Add` in `startFlushLocked` therefore only happens with no `Wait` in progress, which is the `sync.WaitGroup` reuse rule.
- `Close` shares the helper and benefits; no Close-specific behavior change.
- Spec gains a scenario for two concurrent Sleeps.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_windowcounter_sync-flush`: concurrent Sleep calls all wait for the in-flight flush and a Wake during any pending stop starts nothing.

## Impact

- `windowcounter/limiter.go` (`Limiter` field, `stopFlushAndWait`, `Wake`, `flushAfterTick`).
- New `windowcounter/zzz_concurrent_sleep_stop_test.go`.
- Sibling `windowcounter-flush-dirty-index` edits `flushPendingLocked` in the same file; hunks do not overlap.
