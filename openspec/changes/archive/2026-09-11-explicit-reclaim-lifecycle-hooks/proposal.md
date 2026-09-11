## Why

Yaegi v0.16.1 (Traefik v3.7.11) synthesizes `create func() (any, error)` returns with no methods, so type-switch discovery of `Sleep` / `Wake` / `Close` never matches in production plugins. Compiled `go test` stays green while the four-event lifecycle is inert under the interpreter. Explicit `func()` hooks fire; this change makes that the `Open` contract.

## What Changes

- **BREAKING.** `Open` (table and package) takes a last argument `hooks Hooks` (`Sleep`, `Wake`, `Close` as `func()`). `create` stays `func() (any, error)`.
- Store hooks on the slot at put. Bind and reclaim use those stored funcs and ignore a later `Open`'s hooks argument.
- Nil hook funcs skip that event. Drop `sleeper` / `waker` / `closer` type-switch discovery on the stored value.
- Compiled tests pass `Hooks` (method values / close-overs). Add Yaegi v0.16.1 interpreter tests (test-only module dep) that prove type-switch on create `any` does not match and `Hooks` funcs run.
- `e2e/reclaimprobe` becomes the log-only host: `New` passes slog hooks. Pester keeps put/bind and adds orphan/dispose plus hook-log proof on a cancel/reload path.
- Delete `knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md` when the API lands (this ticket is that take). Usage packet `knowledge/devdocs/std_go_reclaim.md` follows in implement.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_reclaim_value-lifecycle`: Sleep, wake, and close are optional `Hooks` funcs passed to `Open`, not optional interfaces on the stored value. Lookups MUST NOT type-switch the stored `any`. Interpreter tests SHALL observe the four events through those funcs.
- `std_go_reclaim_context-lease`: `Open(ctx, key, logger, create)` grows `hooks`. Incarnation end calls the stored Close hook, not `Close()` on the value. Traefik Yaegi load SHALL run those hooks; they MUST NOT remain inert.

## Impact

- `reclaim/table.go`, `reclaim/default.go`, every `Open` call in `reclaim/table_test.go` (compiled) and `e2e/reclaimprobe/plugin.go`.
- Root `go.mod`: test-only `github.com/traefik/yaegi v0.16.1`. Library packages stay stdlib-only. Do not add Yaegi to `e2e/reclaimprobe/go.mod`.
- `scripts/integration-tests.Tests.ps1` (orphan/dispose + hook logs). Compose still `traefik:v3.7.11`, `useunsafe: false`.
- Future consumers (geoblock import switch) pass explicit hooks. No Traefik/Yaegi upgrade. No `Table[T]`. No grace/context-lease semantic change.
