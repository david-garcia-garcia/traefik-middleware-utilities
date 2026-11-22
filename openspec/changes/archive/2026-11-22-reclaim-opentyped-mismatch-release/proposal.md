## Why

`OpenTyped` calls `OpenWithHooks`, which binds a holder and registers `context.AfterFunc(ctx, drop)`, and only then asserts `stored.(T)`. On a mismatch it returns the zero `T` and an error but keeps the holder. The caller got nothing, yet its context keeps the incarnation awake: Sleep and Close cannot run while that context lives, and every retry with the same context adds another holder. Reproduced: 100 failed `OpenTyped[int]` on a key holding a string leave `holders=100` and Close never runs with grace 0. A realistic trigger is two consumers sharing one `Table` with the same key string and different types.

## What Changes

- The bind path returns an internal lease alongside the value. `OpenWithHooks` ignores it (public signature unchanged). `OpenTyped` releases the lease on a type mismatch.
- Releasing a lease drops that holder exactly once: a per-bind guard ensures that the release and the context watcher (`context.AfterFunc` or the nil-Done `watch`) cannot both drop it. When the watcher is an `AfterFunc`, the release also calls its `stop`.
- After the release, the table behaves as if that `OpenTyped` had not bound: the holder count is back, and if no other holder remains the normal drop path runs (Sleep, or Close at grace 0).
- `create` that already ran is not undone (the create-once contract stays); only this caller's holder goes.
- Signature and error text of `OpenTyped` (`reclaim: open %q: want %T, got %T`) do not change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_reclaim_value-lifecycle`: an `OpenTyped` type mismatch does not keep a holder.

## Impact

- `reclaim/table.go` (`finishBind`, `dropWhenDone`, `watch`, an internal open variant returning the lease), `reclaim/opentyped.go`.
- New `reclaim/zzz_opentyped_mismatch_release_test.go`; `knowledge/devdocs/std_go_reclaim.md` one sentence on `OpenTyped`.
- No change for successful `OpenTyped`, `Open`, or `OpenWithHooks` callers.
