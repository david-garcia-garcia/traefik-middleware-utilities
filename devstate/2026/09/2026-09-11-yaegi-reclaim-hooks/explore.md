# Explore
IssueKey: 2026-09-11-yaegi-reclaim-hooks

Verdict: in progress

## Concepts

The reclaim table stores `any` and drives four events on that value: create, sleep, wake, close. Today sleep/wake/close are optional interfaces (`sleeper` / `waker` / `closer`) looked up with a type switch in `reclaim/table.go` (`sleepValue` 146–151, `wakeValue` 154–159, `closeValue` 162–167). Traefik v3.7.11 loads production plugins through Yaegi v0.16.1 (`docker-compose.yml`; pin `github.com/traefik/yaegi v0.16.1` in traefik@v3.7.11 `go.mod`). Both the plugin and `reclaim/` are interpreted (GOPATH), not compiled into Traefik.

```
  Traefik New(ctx) ──► plugin New ──► reclaim.Open(create func() (any, error))
                                              │
                                              ▼
                                         slot.value any
                                              │
                         type-switch sleeper/waker/closer  ── Yaegi: no match
                                              │
                         explicit func() on the slot        ── Yaegi: runs
```

Usage packet: `knowledge/devdocs/std_go_reclaim.md` (Table, Open, Lifecycle, Sleep, Wake, Grace). Research: `knowledge/research/ext_traefik_plugins_yaegi-generics/`. Spec that will need a delta: `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` requirement “Lifecycle events are optional interfaces on the value” (lines 80–85). Create stays `func() (any, error)` (`std_go_reclaim_context-lease`).

Product callers of `Open` (searched `*.go` for `.Open(` and `reclaim.Open`):

- `reclaim/table.go:180` — `Table.Open`
- `reclaim/default.go:28` — package `Open` forwards
- `e2e/reclaimprobe/plugin.go:54` — only non-test consumer
- `reclaim/table_test.go` — 56 compiled `Open` calls

No identity reconstruction (client address, user, tenant, Host). Holders still pass Traefik `New` ctx.

### Reproduction

Claimed failure **reproduced**.

Ran (worktree `D:/repositories/wt-modsec-2026-09-11-yaegi-reclaim-hooks`):

1. Compiled: `go test ./reclaim/ -count=1` — **pass** (1.075s). `TestTable_LifecycleIsCreateSleepWakeClose` still sees sleep/wake/close on a compiled `lifecycle` value (`reclaim/table_test.go:346`).
2. Interpreted: throwaway GOPATH module (`%TEMP%/yaegi-reclaim-hooks-probe`, not in the product tree). `github.com/traefik/yaegi v0.16.1`, `interp.New(Options{GoPath: plugins-local})`, `Use(stdlib.Symbols)` only (no unsafe; matches `docker-compose.yml` `useunsafe: false`). Junction of this worktree at `plugins-local/src/github.com/david-garcia-garcia/traefik-middleware-utilities`. Interpreted `hookprobe` imported this `reclaim`.

Interp results (extract `knowledge/research/ext_traefik_plugins_yaegi-generics/.sources/yaegi-reclaim-hooks-probe.md`):

| Probe | Result |
|---|---|
| `create func() (any, error)` then type-switch to `sleeper` | `switch-sleeper=no`; `%T` = `*struct { Xsleeps atomic.Int32; ... }`; reflect methods `[]` |
| comma-ok `value.(sleeper)` on that return | `ok=false` (no panic on this path) |
| concrete `value.(*life)` | `ok=true`; `%T` still the synthesized struct |
| `reclaim.NewTable(20ms)` + `Open` returning `*life` with Sleep/Wake/Close | logs `reclaim_put` / `reclaim_bind` / `reclaim_orphan` / `reclaim_dispose`; **sleeps=0 wakes=0 closes=0** |
| stored `func()`, same-package hooks struct, extra Open-like `func()` params | all fired (counts 1) |
| cross-package `hooklib.Hooks{Sleep: value.Sleep, ...}` and extra `func()` args | both fired |
| `func() (any, Hooks, error)` same-package and cross-package | worked |

So: table state machine and logs run under Yaegi; optional methods on the stored value do not. Explicit `func()` hooks do. Pester e2e was not re-run; `e2e/reclaimprobe/plugin.go` has no lifecycle methods, and `scripts/integration-tests.Tests.ps1` only asserts `reclaim_put` / `reclaim_bind`.

## Decisions

**Change `Open` to take a `Hooks` value, not extra positional funcs and not a create-return of hooks.** `type Hooks struct { Sleep, Wake, Close func() }` in `reclaim`. Signature:

```
Open(ctx, key, logger, create func() (any, error), hooks Hooks) (any, error)
```

`create` stays `func() (any, error)` (Yaegi still cannot take create args; live spec requires that shape). Nil hook funcs skip that event (optional). Store hooks on the slot at **put**; bind and reclaim use the stored hooks and ignore the argument (a later `New` must not replace the incarnation’s Sleep/Wake/Close). Callers construct the value inside `create` and close over it:

```
var v *BIN
reclaim.Open(ctx, key, logger, func() (any, error) {
    v = newBIN(cfg)
    return v, nil
}, reclaim.Hooks{Sleep: func() { v.Sleep() }, Wake: func() { v.Wake() }, Close: func() { v.Close() }})
```

Both a `Hooks` struct and extra `func()` parameters fire under Yaegi v0.16.1 (cross-package). The struct is one job: the three funcs always travel together (`skill:opd-commandments:One job, one owner`). `create func() (any, Hooks, error)` also works interpreted but would change create, which the requirement did not ask to change and the context-lease spec forbids.

**Drop type-switch discovery.** No convenience wrapper that still asserts `sleeper` / `waker` / `closer` on the stored `any`. Desired says explicit hooks, not optional interface discovery. Compiled tests pass `life.Sleep` as hook funcs. `sleepValue` / `wakeValue` / `closeValue` become calls of the stored funcs. Take (delete) `knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md` when that lands.

**Yaegi tests import `github.com/traefik/yaegi v0.16.1` as a test-only module dependency.** Not vendored into library runtime (`go.mod` of `reclaim/` stays stdlib-only). Harness: `interp` + GOPATH + `stdlib.Symbols`, `useunsafe` false. Prove (1) type-switch on create `any` does not match, (2) `Hooks` funcs run through `Open` (sleep/wake/close counts). Lean coverage in `go test`; do not start Traefik there.

**Test host is `e2e/reclaimprobe` with log-only hooks.** `New` passes `Hooks` that slog sleep/wake/close. Pester stays the Traefik wrap-up: keep put/bind; add orphan/dispose and hook-log proof on a cancel/reload path (whoami recreate / second `New`, same key). Do not add Yaegi to `e2e/reclaimprobe/go.mod`.

**Spec delta on `std_go_reclaim_value-lifecycle`:** replace “optional interfaces on the value” with optional `Hooks` funcs passed to `Open`, still type-switch-free, still optional. Context-lease `Open(ctx, key, logger, create)` grows `hooks`. Usage `knowledge/devdocs/std_go_reclaim.md` follows in implement. Archived Non-Goal “explicit hooks” is this ticket’s job, not a deviation from this ask.

## Open questions

- Q: Exact explicit-hook API shape on `Open` (function parameters vs struct)?
  Rank: bounded asked — existing `Open` contract; callers enumerated: `table.go:180`, `default.go:28`, `e2e/reclaimprobe/plugin.go:54`, 56 `Open` calls in `reclaim/table_test.go` (searched `*.go` for `.Open(`); all migrate here. Desired: “Change `Open` to take explicit lifecycle hooks”
  Decision: resolved — last argument `reclaim.Hooks{Sleep, Wake, Close func()}`. Store on the slot at put; bind/reclaim ignore the new argument. Nil funcs skip. Callers close over the pointer set inside `create`.
  By: propose

- Q: Do compiled callers keep optional methods on the value as a convenience wrapper, or migrate fully to explicit hooks?
  Rank: bounded asked — Desired: “not optional interface discovery on the stored value”; lookups are `sleepValue`/`wakeValue`/`closeValue` in `reclaim/table.go:146-167` (same file as Open; Reset/dispose call them)
  Decision: resolved — full migrate. Delete `sleeper`/`waker`/`closer` type-switches. Compiled tests pass `Hooks{Sleep: life.Sleep, ...}`.
  By: propose

- Q: Which Yaegi import path and version pin matches Traefik v3.7.11 for unit tests?
  Rank: additive asked — new test dependency this change creates; Desired: “Add tests importing the actual Yaegi interpreter”; Out of scope: runtime vendor
  Decision: resolved — `github.com/traefik/yaegi v0.16.1` (traefik@v3.7.11 `go.mod`). Test files only. GOPATH interp + `stdlib.Symbols`. No `unsafe`.
  By: propose

- Q: Where do Yaegi interpreter tests live, and what is the test host for all four hooks?
  Rank: additive asked — Desired: interpreter tests plus “a test host for the reclaim table that exercises all lifecycle hooks (likely log-only)” and Pester wrap-up
  Decision: resolved — `reclaim/*_yaegi_test.go` (or `reclaim/yaegi_test.go`) imports yaegi; GOPATH consumer under testdata/temp. `e2e/reclaimprobe` is the log-only host (explicit hooks + slog). Pester: put/bind plus orphan/dispose and hook log lines on a reload path (stop both whoami holders, then start within grace).
  By: propose
