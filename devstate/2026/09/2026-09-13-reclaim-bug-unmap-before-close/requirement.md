# Requirement
IssueKey: 2026-09-13-reclaim-bug-unmap-before-close

## Problem
Zero grace (and expire after a positive grace) unmaps the key, then runs Close outside `t.mu`. A concurrent `Open` sees the key absent and `create`s the next incarnation while the previous Close is still in flight. Reproduced 5/5 with `NewTable(0)`, a Close hook that signals then blocks, cancel of the last holder, and `Open` of the same key.

## Current (code)
- `reclaim/table.go` `drop` — after `runSleep`, publishes `slotAsleep` and `close(ready)`. Zero grace (`grace <= 0`) `delete`s the key under `t.mu` before Close, then calls `expire`.
- `reclaim/table.go` `expire` — requires `slotAsleep` and `holders == 0`, sets `slotGone`, `delete`s the key if still mapped, unlocks, then `dispose` (`runClose` then `reclaim_dispose`). Close therefore runs with the key already absent.
- `reclaim/table.go` `Open` — unmapped key registers `slotBusy` and `put`/`create`. `slotBusy` waiters block on `ready` then look again. After unmap they take the create path, not reclaim.
- `reclaim/table.go` `slotGone` comment — claimed for close (or create failed) and the key is already unmapped.
- `reclaim/table.go` `Reset` — replaces `t.items` first (unmap), then Sleep/Close for awake/asleep outside `t.mu`. Tests-only; must not race `Open`.
- `reclaim/table.go` `runClose` / `dispose` — Close already runs outside `t.mu`.
- `reclaim/table_test.go` `TestTable_ZeroGraceEndsImmediately` — zero grace Sleep then Close; key not stored after dispose. Does not assert Close-vs-create overlap.
- `reclaim/table_test.go` `TestTable_ZeroGraceRacingOpenIsPlainBind` — second `Open` during Sleep; after Sleep the new value is a create (`reclaim_put`/`reclaim_bind`, no reclaim). Does not fail if that create starts while Close of the first incarnation is still blocked.
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` — Open waits while another Open, sleep, or create is in flight. Zero grace: sleep then close, key no longer stored. Does not name Close as a wait-for transition, and does not forbid unmap-then-Close.
- `openspec/specs/std_go_reclaim_context-lease/spec.md` — zero grace disposes as soon as the last holder is gone, with no sleeping window. Close hook must return before `reclaim_dispose`. Does not require the key to stay mapped until Close returns.
- Dest has no test that fails when create of incarnation 2 starts while Close of 1 is blocked.

## Desired
Make the Close-before-create wait OPT-IN per incarnation via `Hooks.EnforceCloseBeforeOpen` (default false = dest: unmap then Close, overlap allowed).
1. Keep the wait tests, but they set `EnforceCloseBeforeOpen: true`. Add a deterministic default-off test: Close signals then blocks; `Open` MUST return a fresh value while Close is still blocked.
2. When the stored flag is true: keep the key mapped `slotBusy` for the whole Close, then unmap and `close(ready)`.
3. When the stored flag is false: dest — unmap, then Close. A concurrent `Open` may create during Close.
4. Read the flag from the ending incarnation's stored hooks, never from a later `Open`.
5. Do not run Close under `t.mu`. Do not leave the slot asleep during Close. Close always goes through `runHook`/`dispose` so a panic cannot escape AfterFunc, on both flag paths.
6. Reset stays unmap-first regardless of the flag.

## Affected
- `reclaim/table.go` (`drop` zero-grace unmap, `expire` unmap-then-Close, slot state during Close)
- `reclaim/table_test.go` (failing-then-passing overlap test; existing reclaim tests stay green)
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` (Close as a wait-for transition; unmap after Close)
- `openspec/specs/std_go_reclaim_context-lease/spec.md` (key stays mapped until Close returns; no sleeping window at zero grace)

## Out of scope
- Canceled-ctx disposed return
- Hook panic bricks the key
- Other reclaim bugs
- Running Close under `t.mu`
- Leaving the slot asleep during Close so a racing `Open` can reclaim

## Unknowns
- Whether Reset’s Close-before-unmap change is cheap enough to keep existing Reset tests (`TestTable_ResetDuringSleepStillOrphansBeforeDispose`, `TestTable_ResetRacingADropKeepsOrphanBeforeDispose`, `TestTable_ResetRacingOpenClosesEveryValue`, and siblings).
- Whether an expire-after-positive-grace overlap test is cheap enough to land next to the zero-grace test.
- Test function name and exact wait budget (ticket gave a 200ms wait-for-Close example).

## Tensions
- Ticket: Close of an incarnation must finish before the key is absent for a new create. Dest `drop`/`expire` unmap then Close.
- Spec already requires Open to wait for in-flight Open, sleep, or create, and requires Close to return before `reclaim_dispose`. It does not list Close as a wait-for transition and does not say the key stays mapped until Close returns. The agreed how adds that; it does not add a sleeping window at zero grace (`slotBusy`, not `slotAsleep`).
- `TestTable_ZeroGraceRacingOpenIsPlainBind` still wants a new create after zero-grace Sleep (no reclaim). After the fix that create must start only after Close of the previous incarnation returns; the log sequence `put`/`bind`/`orphan`/`dispose` then `put`/`bind` stays the desired record.
- Ticket: do not invent another how. Keep mapped `slotBusy` through Close, then unmap / `close(ready)`. Do not fix the other two reclaim bugs.
