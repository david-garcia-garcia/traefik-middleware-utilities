## Context

See proposal.md for why. Live spec: `std_go_reclaim_context-lease`. Usage: `knowledge/devdocs/std_go_reclaim.md` (enough to call `Open`; AfterFunc is an internal wait). Research: `knowledge/research/ext_traefik_plugins_yaegi-afterfunc/` (Yaegi v0.16.1 maps AfterFunc; GOPATH interp call passed). Explore: `devstate/explore.md`.

Today `put`, bind (`Open` `slotAwake`), and `reclaimLocked` each `go t.watch`. `watch` calls `waitCtx` then `drop`. `waitCtx` blocks on `<-done` when `Done() != nil`, else polls `Err()` every 20ms. `drop` returns when `incarnation.state != slotAwake` (`reclaim/table.go`). `go 1.21`. AfterFunc is not used in this tree yet.

## Goals / Non-Goals

**Goals:**
- `Done() != nil` → `context.AfterFunc(ctx, drop)` so a cancellable hold does not park a waiter.
- `Done() == nil` → keep `go watch` / `waitCtx` poll.
- Compiled hold-time goroutine coverage for cancellable holders. Existing `reclaim/yaegi_test.go` GOPATH harness (`stdlib.Symbols`, `useunsafe` false) still loads `table.go`.

**Non-Goals:**
- An AfterFunc SHALL, a wait-mechanism rewrite of the live spec, or any edit of `std_go_reclaim_value-lifecycle`.
- Storing AfterFunc's stop func. Changing `Open`, grace, hooks, logging, or create. A new Pester AfterFunc case. Upgrading Traefik or Yaegi.

## Decisions

1. **Branch at the three bind sites (or one helper they all call).** `Done() != nil`: `context.AfterFunc(ctx, func() { t.drop(key, incarnation) })`. `Done() == nil`: `go t.watch` as today. Alternative: AfterFunc for every holder — rejected: AfterFunc never runs `f` when `Done()` is nil (`nilDoneCtx`, `Background`). Alternative: keep `go t.watch` at the sites and put AfterFunc inside `watch` — worse: `watch` would return immediately on the cancellable path, so the sites would need to drop the `go` anyway. One helper keeps bind/put/reclaim symmetrical.

2. **Register `drop`. Do not store the stop func.** `drop` already no-ops when `state != slotAwake`. `Reset` marks awake slots gone. The stale-holder test already requires a late fire not to sleep the next incarnation. Stop storage would be a new per-holder field. AfterFunc still `go f()` at cancel (or immediately if already done); that goroutine must still exit.

3. **Leave `waitCtx` as the poll waiter.** Do not restyle it. `watch` stays the nil-Done path. The `<-done` branch can remain; it is not the cancellable bind path after this change.

4. **Prove in `reclaim` tests, not Pester.** Add compiled coverage that cancellable holders do not grow goroutines during the hold (`settledGoroutines` already used by `TestTable_GoroutinesReturnToBaseline`, which stays as the post-cancel case). Keep `TestTable_HolderWithoutDoneChannelIsPolled`. Existing Yaegi tests copy non-test `.go` into GOPATH and `Open` with `WithCancel`; they must still load `table.go`. No new interp-only AfterFunc probe and no new Pester case. Pin stays v0.16.1.

5. **No usage-packet write.** Callers still pass Traefik `New` ctx into `Open`. AfterFunc is not a caller-facing contract.

## Risks / Trade-offs

- [Interpreted `table.go` fails to load AfterFunc even though the throwaway probe called it] → Mitigation: existing `yaegi_test.go` GOPATH copy of `table.go` is the product proof; pin stays v0.16.1; `useunsafe` false.
- [Already-canceled holder] → AfterFunc runs `f` immediately in its own goroutine. Same drop as today's `watch` returning at once from `<-done`.
- [Late AfterFunc after `Reset` / next incarnation] → Mitigation: `drop` no-ops when `state != slotAwake`; keep `TestTable_ResetLogsOrphanThenDisposeAndKeepsNextIncarnation`.
- [Hold-time goroutine assert flakes on this host] → Mitigation: reuse `settledGoroutines` and the same slack as `TestTable_GoroutinesReturnToBaseline`.

## Migration Plan

Library-only wait change in `reclaim/`. Apply updates `table.go` and compiled tests. Rollback is revert the branch. No API or e2e host change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
