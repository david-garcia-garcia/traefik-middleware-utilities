## Context

See proposal.md for why. Live specs: `std_go_reclaim_value-lifecycle`, `std_go_reclaim_context-lease`. Usage: `knowledge/devdocs/std_go_reclaim.md` (implement updates Language and the Open snippet). Research: `knowledge/research/ext_traefik_plugins_yaegi-generics/` (Yaegi v0.16.1; type-switch on create `any` does not match; explicit `func()` hooks fire, including a cross-package `Hooks` struct). Explore decisions: `devstate/explore.md`. Callers of `Open` today: `reclaim/table.go`, `reclaim/default.go`, `e2e/reclaimprobe/plugin.go`, compiled `reclaim/table_test.go`. Two whoami routes share key `shared`; both holders must go Done before orphan.

## Goals / Non-Goals

**Goals:**
- One `Hooks` value on `Open`; store it on the slot at put; drive sleep/wake/close from those funcs only.
- Compiled tests and a GOPATH Yaegi interp test cover the contract. Pester proves hooks under Traefik on a two-holder teardown/reload.

**Non-Goals:**
- Extra positional `func()` parameters, `create func() (any, Hooks, error)`, or a compiled convenience wrapper that still type-switches the stored `any`.
- Vendoring Yaegi into library runtime imports. Adding Yaegi to `e2e/reclaimprobe/go.mod`.
- Changing grace, holder, or logging semantics. Upgrading Traefik or the Yaegi pin.

## Decisions

1. **`type Hooks struct { Sleep, Wake, Close func() }` as the last `Open` argument.** Alternative: three extra `func()` parameters (also fires interpreted) — rejected: the three funcs always travel together. Alternative: `create` returns hooks — rejected: live spec keeps `func() (any, error)`; Yaegi cannot give `create` arguments. Callers close over a pointer assigned inside `create`.

2. **Store hooks at put; bind/reclaim ignore the new argument.** Alternative: replace hooks on every `Open` — rejected: Sleep/Wake/Close belong to the incarnation, not the latest holder. Nil funcs skip.

3. **Delete `sleeper` / `waker` / `closer` and the type-switch helpers.** `sleepValue` / `wakeValue` / `closeValue` (and `dispose`) take the stored `Hooks` (or the three funcs) and call when non-nil. Compiled tests pass method values (`Hooks{Sleep: life.Sleep, ...}`) or a test-only helper that binds those methods. Production MUST NOT look at the stored `any` for lifecycle.

4. **Yaegi tests live in `reclaim/` (`yaegi_test.go` or `*_yaegi_test.go`).** Require `github.com/traefik/yaegi v0.16.1` on the module `go.mod` (test files only). Harness: `interp.New(Options{GoPath})`, `Use(stdlib.Symbols)`, no `unsafe`. GOPATH consumer in `t.TempDir()` or `reclaim/testdata/` that imports this module's `reclaim`. Prove type-switch miss and hook counts. Do not start Traefik in `go test`.

5. **Test host is `e2e/reclaimprobe`, log-only.** `New` constructs the stored value inside `create` and passes `Hooks` that slog stable probe messages (names say probe). Do not add Yaegi to the probe module. Pester: keep put/bind and shared identity. For orphan/reclaim, stop **both** whoami services (two holders share `shared`) then start them again within `DefaultGrace` (10s). For close, leave them down past grace. Assert table msgs plus the probe hook lines.

6. **Take the debt file in the same apply** that lands hooks. Update `knowledge/devdocs/std_go_reclaim.md` in implement (Open signature, Sleep/Wake as hooks, snippet close-over, drop the "optional methods do not run under Yaegi" gotcha). Close `issues.md` / progress notes only if that file exists for this run; the debt path is `knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md`.

7. **`table.go` stays stdlib-only.** `TestTable_StdlibImports` still holds. Yaegi is a test import of the module, not of `reclaim/table.go`.

## Risks / Trade-offs

- [Two-holder e2e never orphans if only one whoami restarts] → Mitigation: stop/start both whoami services; do not restart a single route.
- [Pester waits `DefaultGrace` (~10s) for dispose] → Mitigation: one teardown test with a timeout above 10s; do not add `NewTable` to the probe.
- [Yaegi GOPATH cannot resolve this module from `t.TempDir()`] → Mitigation: junction or copy the worktree (or `reclaim/` + `go.mod`) into `plugins-local/src/github.com/david-garcia-garcia/traefik-middleware-utilities`, matching compose.
- [Root `go.mod` grows Yaegi transitives; authors think runtime depends on it] → Mitigation: only `*_test.go` imports yaegi; `table.go` import test stays green; README/usage say test-only.
- [Callers forget to close over the pointer and capture a nil] → Mitigation: usage snippet shows `var v *T` assigned inside `create`; compiled tests use method values after create returns the same pointer.

## Migration Plan

Library has no production consumer besides `e2e/reclaimprobe`. Apply updates `Open`, tests, probe, Pester, usage, and deletes the debt file. Rollback is revert the branch. Geoblock import switch is a later consumer change; it will pass `Hooks`.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
