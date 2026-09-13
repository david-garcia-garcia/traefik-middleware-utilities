# Explore
IssueKey: 2026-09-13-reclaim-bug-nildone-watcher-leak

## Concepts

```
  nil-Done holder (Background / Yaegi)     dest today
  ------------------------------------     ----------
  Open ── dropWhenDone ── go t.watch
                              │
                         waitCtx polls Err @ 20ms
                              │
         ctx.Err() never set ─┘  (only exit)
                              │
  Reset / expire / endBusySlot / unmapAfterClose
  set slotGone, Sleep, Close, unmap
                              │
                         watcher still in waitCtx
                         retains key + *slot + value
```

Agreed how: give `watch` a second exit — stop polling once this incarnation is over — and return **without** `drop`. A holder cannot be dropped from a slot that is already gone. `ctx.Err()` still drops (keep `TestTable_HolderWithoutDoneChannelIsPolled`).

Two ticket shapes. This run takes **shape 1** (per-incarnation `finished` channel).

```
  slot.finished  (open at put; close exactly once)
       │
       ├─ endBusySlot          (create fail; Sleep panic; Wake panic)
       ├─ unmapAfterClose      (endMappedClose: enforce Close-then-unmap)
       ├─ expire non-enforce   (unmap-then-Close)
       └─ Reset                (awake / asleep only; busy left to its owner)
```

## Current (measured)

- `reclaim/table.go` `waitCtx` — `Done() != nil` waits on the channel; else `for ctx.Err() == nil { time.Sleep(20 * time.Millisecond) }`. No incarnation-end exit.
- `dropWhenDone` — `Done() != nil` → `context.AfterFunc` → `drop`; else `go t.watch`. `watch` always `drop`s after `waitCtx`.
- `requireContext` rejects only `ctx == nil`. `TestTable_OpenBackgroundDoesNotPanic` accepts `context.Background()`.
- `slotGone` is assigned in exactly four functions: `endBusySlot`, `unmapAfterClose`, non-enforce `expire`, and `Reset` (awake/asleep). `endMappedClose` only calls `unmapAfterClose`. Sleep-panic `drop` and Wake-panic `reclaimLocked` already call `endBusySlot` (do not edit those panic bodies — parallel ticket `2026-09-13-reclaim-bug-panic-ignores-enforce`).
- `Reset` for `slotBusy` / already-`slotGone` leaves the ending to the owner goroutine. Closing `finished` in Reset for those states would double-close when the owner later hits `endBusySlot` / `unmapAfterClose` / `expire`.
- Spec `std_go_reclaim_context-lease`: nil-Done holders stay live until `ctx.Err()` is set; every table goroutine SHALL exit once holders are Done **and** the incarnation has ended; a stale holder drop MUST NOT change a later incarnation.
- Dest constructor is `New(Config)` (`c960bfe`). Production `NewTable` is gone. The validated reproducer at the caller checkout calls `NewTable(time.Millisecond)` and will not compile on dest until a test-only wrapper exists. Helpers it names (`recHandler`, `recLogger`, `nilDoneCtx`, `waitUntil`, `countMsg`, `settledGoroutines`, `MsgDispose`, `lifecycle`) are in dest `reclaim/table_test.go`. Dest `reclaim/` has no `repro_nildone_watcher_test.go`.
- Usage `knowledge/devdocs/std_go_reclaim.md` does not say a nil-Done watcher exits when the incarnation ends. Do not rewrite it in explore. Propose + devdocsimpact update it with the change.
- No third-party research: in-tree `reclaim` plus stdlib `context`. Identity (client address / tenant / Host) is not in play.

## Decisions

- Shape 1: park `finished chan struct{}` on `slot`, created next to `ready` at first map. Close it exactly once, under `t.mu`, on the four `slotGone` writers. `watch` `select`s on `finished` plus the 20 ms tick; if `finished` wins, return without `drop`. If `ctx.Err()` is set and `finished` is still open, `drop` as today.
- Do not take shape 2 (`t.mu` + `slotGone` every tick): it still works, but it takes the mutex at 50 Hz per holder for the whole poll, including after the incarnation has ended until the next tick notices. Shape 1 waits with no lock held. Ticket prefers 1.
- Close-once: helper that closes then nils `finished`. Call it only while holding `t.mu`. `Reset` calls it for awake/asleep in the same lock that sets `slotGone`. Skip busy/gone on Reset.
- Do not reject Background. Do not shorten the 20 ms tick. Do not edit `reclaim/BUGS.md`. Do not touch Sleep-panic / Wake-panic branch bodies.
- Tests first: copy the caller reproducer verbatim into `reclaim/repro_nildone_watcher_test.go`. Add a test-only `NewTable` in `table_test.go` so that file compiles on dest (`New(Config{Grace: grace})`). Do not restore production `NewTable`.

## Open questions

- Q: Which watch-exit shape does this run build?
  Rank: bounded asked — changes existing `slot` and four ending functions; enumerated `slotGone` writers in `reclaim/table.go` (`endBusySlot`, `unmapAfterClose`, `expire`, `Reset`) and their callers (`put` create-fail, `drop` Sleep-panic, `reclaimLocked` Wake-panic, `endMappedClose`, zero-grace `drop`, `waitGraceOrWake` timer); Desired #3 names pick one of two shapes
  Decision: assumed — shape 1, per-incarnation `finished` channel, because it does no work while idle and the ticket marks it preferred. Shape 2 is the fallback only if a double-close cannot be made exact.
  By: explore

- Q: Does `endMappedClose` need its own `finished` close?
  Rank: additive asked — Unknowns names `endMappedClose`; it already calls `unmapAfterClose`
  Decision: assumed — no. Close `finished` in `unmapAfterClose` only. Same for Sleep/Wake panic: they already call `endBusySlot`.
  By: explore

- Q: How is `finished` closed exactly once when Reset races a busy owner?
  Rank: bounded asked — 4 `slotGone` writers enumerated; double `close` panics; Desired #3 names every ending path
  Decision: assumed — `closeFinished` under `t.mu` (close then nil). `Reset` signals only awake/asleep. Busy/gone stay with the owner (`endBusySlot` / `unmapAfterClose` / `expire`).
  By: explore

- Q: The verbatim reproducer calls `NewTable`, which dest removed. How does it compile?
  Rank: additive asked — Desired #1 names land the existing file verbatim; spec `std_go_reclaim_context-lease` forbids production `NewTable`
  Decision: assumed — keep the reproducer bytes as on the caller checkout. Add test-only `NewTable` in `reclaim/table_test.go` wrapping `New(Config{Grace: grace})`. Do not add it to `table.go`.
  By: explore

- Q: Do we add a spec scenario for Reset-vs-nil-Done leak, or is Goroutines do not outlive the incarnation enough?
  Rank: additive asked — Desired names the spec may gain a scenario; existing scenario already says holders Done and incarnation ended
  Decision: assumed — add one scenario on `std_go_reclaim_context-lease` that names a never-canceled nil-Done holder plus `Reset`: watchers MUST exit and MUST NOT `drop`. Do not change the existing goroutine-exit SHALL.
  By: explore
