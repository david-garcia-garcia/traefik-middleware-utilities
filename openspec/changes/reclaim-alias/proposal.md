## Why

Downstream middleware (crowdsec bouncer) vendors `traefik-middleware-utilities` with a local `reclaim/` delta: weak alias watchers, `Peek`, and alias cleanup when an incarnation ends. That behavior is not on `master`, so consumers cannot upgrade the module without re-applying a fork. This PR upstreams the measured vendor diff with tests so `master` matches what production already relies on.

## What Changes

- Add `reclaim/alias.go`: `SetAlias`, `Watch`, `ClearPublisher`; types `Watcher`, `Box`, `Published`; alias graph and delivery helpers.
- Extend `reclaim/table.go`: `Table.aliases`, per-slot reverse links, `Peek(key)` returning `(value, State, ok)`, and `unbindIncarnationLocked` on every incarnation teardown path (`endBusySlot`, `unmapLocked`, failed `installCloser`, grace `expireDispose`, `takeAll` / `Reset`).
- Replace the ad-hoc vendor banner on `alias.go` with normal package/API documentation; keep Yaegi constraint comments on `Box`, `Published`, and hook reentrancy.
- Add extensive compiled tests (and selective Yaegi cases) for alias, `Peek`, and teardown unbind; no changes to `opentyped.go` or other packages.
- Update live spec `std_go_reclaim_value-lifecycle` and defer usage-doc Language to devdocsimpact.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_reclaim_value-lifecycle`: public alias weak refs (`SetAlias` / `Watch` / `ClearPublisher`), non-binding `Peek`, and alias watcher reset when an incarnation ends.

## Impact

- `reclaim/alias.go` (new), `reclaim/table.go` (modify).
- New or extended tests under `reclaim/` (`alias_test.go`, `peek_test.go`, optional `yaegi_test.go` cases).
- Live spec delta under this change; `knowledge/devdocs/std_go_reclaim.md` in a later devdocsimpact phase.
- `e2e/reclaimprobe` unchanged (still `Open` only). No module tag or version bump in this PR. Bouncer repo out of scope.
