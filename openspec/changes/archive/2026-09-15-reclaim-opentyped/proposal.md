## Why

`Table.Open` takes lifecycle hooks beside `create`. Hooks are evaluated before `Open` runs, so a caller whose Sleep, Wake, or Close must act on the value being created has to declare a variable, assign it inside `create`, and close the hook funcs over it. That boilerplate was copied at three consumer sites. `EnforceCloseBeforeOpen` is a bool stored at put: an external helper cannot fake it, so the hooks that `create` builds have to reach `publishPut` inside the package.

## What Changes

- Add `Table.OpenWithHooks` whose `create` returns `(any, Hooks, error)`. The lookup loop moves here. `put` takes that create signature, drops its separate hooks argument, and stores the hooks `create` returns.
- Keep `Open` as a backward-compatible wrapper: same signature, same semantics, same nil-argument error precedence (nil table, then nil logger, then nil create). Because the wrapping closure is never nil, `Open` checks its own `create` for nil before delegating and still reports table and logger first.
- Add package-level `OpenTyped[T any]` in `reclaim/opentyped.go`. It calls `OpenWithHooks` and returns the stored value as `T`, or the zero `T` and `reclaim: open %q: want %T, got %T` on a type mismatch.
- `Table` stays non-generic. The generic instantiation of `OpenTyped` MUST remain a call expression in a package that can name `T` (Yaegi v0.16.1).
- Tests for both new functions and a regression that `Open`'s nil-argument errors are unchanged. Yaegi harness case for an `OpenTyped` call expression.
- Update the live hooks contract and the usage packet so they describe the two new functions. No git tag and no module version bump.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_reclaim_value-lifecycle`: hooks MAY arrive from `create` via `OpenWithHooks` or `OpenTyped`; `Open` still accepts hooks beside `create`. Stored-at-put and later-Open-does-not-replace remain. Nil-argument error precedence of `Open` is unchanged.

## Impact

- `reclaim/table.go` (`Open` wrapper, `OpenWithHooks`, `put` create signature).
- New `reclaim/opentyped.go`.
- `reclaim/table_test.go` (and/or a sibling test file): OpenWithHooks, OpenTyped, Open nil-precedence regression.
- `reclaim/yaegi_test.go`: call-expression `OpenTyped` case.
- Live spec `std_go_reclaim_value-lifecycle`.
- Usage packet `knowledge/devdocs/std_go_reclaim.md`.
- No `Open` signature change. No `Table[T]`. No tag or version bump. In-repo `e2e/reclaimprobe` stays on `Open`.
