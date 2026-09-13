## Context

See proposal.md for why. Live specs: `std_go_reclaim_value-lifecycle`, `std_go_reclaim_context-lease`. Usage: `knowledge/devdocs/std_go_reclaim.md`. Explore: `devstate/explore.md`.

`drop` after Sleep publishes `slotAsleep`, `close(ready)`, and at zero grace `delete`s, then `expire`. `expire` requires `slotAsleep`, sets `slotGone`, `delete`s, unlocks, then `dispose` (`runClose` then the dispose log). `Open` creates when the key is absent. `slotBusy` waiters never see Close. Package comment on `Table` already claims Close runs outside `t.mu` with the slot parked in `slotBusy`; dest Close does not.

`dispose` today is Close then the dispose log. After this change Close and the log stay in that order, but unmap sits between them. Do not call `dispose` as a unit if that would Close twice.

## Goals / Non-Goals

**Goals:**
- Close-before-unmap on the zero-grace drop path and on expire after a positive grace.
- Keep Close outside `t.mu`. Keep the Close window non-reclaimable (`slotBusy`, not `slotAsleep`).
- Fail-then-pass compiled tests for both holes. Existing reclaim tests stay green.

**Non-Goals:**
- Reordering tests-only `Reset` (unmap-first stays; must not race `Open`).
- Close under `t.mu`. A sleeping window at zero grace. Fixing canceled-ctx disposed return or hook-panic bricks the key.
- `Open` signature, Yaegi, Pester, or grace duration semantics besides the Close window.

## Decisions

1. **Zero grace stays on the Sleep `ready` through Close.** After Sleep, do not publish `slotAsleep`, do not `close(ready)`, do not unmap. Run Close as part of that same `slotBusy` transition, then unmap and `close(ready)`, then the dispose log. Alternative: publish `slotAsleep` then immediately expire — rejected: that is a sleeping window, and Open's `slotAsleep` path would reclaim. Alternative: `slotGone` while still mapped — rejected: ticket named `slotBusy`; Open already waits on `slotBusy`; `slotGone` is the create-failed / already-unmapped claim.

2. **expire takes `slotBusy` for Close, then unmaps.** Under `t.mu`, if still `slotAsleep` and `holders == 0`, set `slotBusy` and a new `ready`, unlock, `runClose`, then lock, `delete` if still this incarnation, `close(ready)`, unlock, dispose log. Alternative: stay `slotAsleep` during Close — rejected: a racing Open would reclaim the value being closed. Alternative: unmap then Close — dest bug.

3. **Tests first, then the table change.** Add `TestTable_ZeroGraceCreateWaitsUntilPreviousCloseReturns` and `TestTable_ExpireCreateWaitsUntilPreviousCloseReturns` that `t.Fatal` if `create` of incarnation 2 runs while Close of 1 is blocked (200ms wait while Close is held). They MUST fail on dest. Then implement. Then they pass: second Open stays parked until Close returns, then creates. Keep `TestTable_ZeroGraceRacingOpenIsPlainBind` (no reclaim; after the fix, dispose precedes the second put).

4. **Leave `Reset` unmap-first.** `Reset` replaces `t.items` then Sleep/Close. `TestTable_ResetDuringSleepStillOrphansBeforeDispose`, `TestTable_ResetRacingADropKeepsOrphanBeforeDispose`, and `TestTable_ResetRacingOpenClosesEveryValue` depend on that. Production never calls `Reset`. Spec allows tests-only Reset to unmap first.

5. **Usage packet after apply.** `knowledge/devdocs/std_go_reclaim.md` must say Open waits on Close for that key (Language Close / Gotchas). Not a caller API change.

## Risks / Trade-offs

- [Zero-grace racing Open during Sleep currently parks on `ready`; closing `ready` after Sleep would let it create during Close] → Mitigation: do not `close(ready)` until after Close and unmap.
- [expire vs reclaim race at grace end] → Mitigation: both take the slot under `t.mu`; reclaim requires `slotAsleep`; expire switches to `slotBusy` before Close; one wins.
- [Open `slotBusy` path reads `createErr` on the old slot after `ready`] → Mitigation: Close path MUST NOT set `createErr`; loop then sees the key absent and creates.
- [Calling `dispose` after already running Close] → Mitigation: Close then unmap then dispose log; do not invoke `dispose` as a second Close.
- [200ms wait flakes if Close is not entered] → Mitigation: wait on `closeEntered` before starting the second Open, same as the ticket example.

## Migration Plan

Library-only. Apply: tests that fail on dest, then `table.go`, then usage packet. Rollback is revert the branch. No API or e2e host change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
