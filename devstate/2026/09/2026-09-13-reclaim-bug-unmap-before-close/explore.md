# Explore
## Concepts

The reclaim table stores one incarnation per key. `Open` binds a holder context. When the last holder is Done, `drop` Sleeps, logs `reclaim_orphan`, then either keeps the value asleep for grace or ends it. `expire` (and zero-grace `drop`) is supposed to Close once, then log `reclaim_dispose`. Traefik reload is cancel of the last `New` ctx, then `Open` of the same key.

Dest `drop` after Sleep publishes `slotAsleep`, `close(ready)`, and at zero grace `delete`s the key in that same lock, then calls `expire`. `expire` requires `slotAsleep`, sets `slotGone`, `delete`s, unlocks, then `dispose` (`runClose` then the dispose log). Close therefore runs with the key already absent. A concurrent `Open` takes the unmapped-key create path. `slotBusy` waiters never see Close, because Close is not a busy transition.

Package comment on `Table` already says create and stored hooks (Wake, Sleep, Close) run outside `t.mu` with the slot parked in `slotBusy`. Dest Close does not. That is the bug, not a new state machine.

```
Dest (overlap)
  Sleep -> slotAsleep + unmap + close(ready) -> expire Close (outside mu)
  Open sees key gone -> create incarnation 2 while Close of 1 still runs

Agreed
  Sleep -> stay slotBusy -> Close (outside mu) -> unmap + close(ready)
  Open waits on slotBusy; after ready the key is gone -> create
```

Positive-grace expire is the same hole after grace elapses: still `slotAsleep` -> `slotGone` + delete, then Close. A racing `Open` during that Close cannot reclaim (key already gone) and instead creates. Switching expire to `slotBusy` for the Close window is what the ticket named so Open cannot reclaim either.

`Reset` is tests-only. It replaces `t.items` first, then Sleep/Close for awake/asleep slots outside `t.mu`. Existing Reset tests (`ResetDuringSleepStillOrphansBeforeDispose`, `ResetRacingADropKeepsOrphanBeforeDispose`, `ResetRacingOpenClosesEveryValue`) depend on that unmap-first plus “leave a busy drop to its owner”. Production Traefik never calls `Reset`.

Usage packet `knowledge/devdocs/std_go_reclaim.md` already names Close-after-Sleep and “Open waits while another Open, sleep, or create is in flight” via the specs. It does not say Open waits for Close, because dest does not.

## Decisions

- How: keep the key mapped `slotBusy` for the whole Close, then unmap and `close(ready)`. Do not publish `slotAsleep` on the zero-grace ending path. expire switches to `slotBusy` (not asleep, not gone-and-deleted) for Close. Close stays outside `t.mu`.
- Tests first: land a product test that fails on dest overlap, then the table change, then it passes. Existing reclaim tests stay green. Do not fix canceled-ctx disposed return or hook-panic bricks key.
- Spec fold: `std_go_reclaim_value-lifecycle` (Close as a wait-for transition; unmap after Close) and `std_go_reclaim_context-lease` (key stays mapped until Close returns; still no sleeping window at zero grace). Fold, not a new leaf.
- Usage: after apply, `std_go_reclaim.md` must say Open waits on Close for that key (Language Close / Gotchas). Not written in explore.

## Open questions

- Q: Should tests-only `Reset` use the same Close-before-unmap ordering?
  Rank: bounded asked — Reset call sites are `reclaim/table.go` plus tests in `reclaim/table_test.go` and `reclaim/default.go` (enumerated); Desired 7 names the reorder only if cheap and existing Reset tests stay green
  Decision: assumed — skip. `Reset` already replaces `t.items` then Sleep/Close; `TestTable_ResetDuringSleepStillOrphansBeforeDispose` and `TestTable_ResetRacingADropKeepsOrphanBeforeDispose` require leaving a busy drop to its owner; `TestTable_ResetRacingOpenClosesEveryValue` races Open vs Reset. Reordering is not cheap. Production never calls `Reset`. Ticket already requires Reset must not race Open.
  By: explore

- Q: Land an expire-after-positive-grace overlap test next to the zero-grace test?
  Rank: additive asked — Desired 1 names cover expire if cheap (same unmap-then-Close in expire)
  Decision: assumed — land it. Same Close-hook-blocks pattern after grace elapsed (`closeEntered`), then concurrent `Open`. Cheap: one test beside the zero-grace case.
  By: explore

- Q: Test function names and wait budget?
  Rank: additive asked — Desired 1 plus the ticket example; existing tests already use `TestTable_` and a short wait that the second Open is still parked
  Decision: assumed — `TestTable_ZeroGraceCreateWaitsUntilPreviousCloseReturns` and `TestTable_ExpireCreateWaitsUntilPreviousCloseReturns`. 200ms wait while Close is blocked (ticket example). `t.Fatal` if create ran during that wait. Then `close(releaseClose)` and the second Open must return without error and create after Close.
  By: explore
