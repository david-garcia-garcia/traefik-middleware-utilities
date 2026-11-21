## Why

Zero grace (and expire after a positive grace) unmaps the key, then runs Close outside `t.mu`. A concurrent `Open` sees the key absent and `create`s the next incarnation while the previous Close is still in flight. Traefik reload is that sequence. Callers whose value owns an exclusive resource need to opt in so Close of an incarnation finishes before the key is absent for a new create. The default stays dest: overlap is allowed.

## What Changes

- Add `Hooks.EnforceCloseBeforeOpen` (bool, default false), stored at put and read from the ending incarnation's stored hooks.
- When the flag is false: dest behaviour — unmap, then Close, a concurrent `Open` may create during Close.
- When the flag is true: keep the key mapped `slotBusy` for the whole Close, then unmap and `close(ready)`. Close still runs through `runHook` / `dispose` so a panicking Close cannot escape on AfterFunc.
- Zero grace with the flag on: after Sleep, do not publish `slotAsleep`. Close as part of that same busy transition, then unmap / `close(ready)`.
- expire with the flag on: switch to `slotBusy` before Close. With the flag off: dest unmap then Close.
- Do not run Close under `t.mu`. Do not leave the slot asleep during Close.
- Tests-only `Reset` stays unmap-first regardless of the flag.
- Close panic recovery from dest stays on both flag paths.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_reclaim_value-lifecycle`: when `Hooks.EnforceCloseBeforeOpen` is set, `Open` waits while Close is in flight for that key. Default (flag unset) allows create during Close.
- `std_go_reclaim_context-lease`: when the flag is set, the key stays mapped until Close returns. Zero grace still has no sleeping window. Default allows overlap. `reclaim_dispose` preceding the next `reclaim_put` holds only when the flag is on.

## Impact

- `reclaim/table.go` (`Hooks.EnforceCloseBeforeOpen`, `drop` / `expire` branch, Close still only via `dispose`/`runHook`).
- `reclaim/table_test.go`: wait tests set the flag; a new default-off test proves Open returns while Close is blocked.
- Live specs `std_go_reclaim_value-lifecycle` and `std_go_reclaim_context-lease`.
- Usage packet `knowledge/devdocs/std_go_reclaim.md`.
- No `Open` signature change. No `NewTable` / `Table` flag. No `Reset` reorder.
