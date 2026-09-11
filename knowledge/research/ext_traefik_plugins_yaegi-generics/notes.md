# Yaegi plugin generics and reclaim table shape

Traefik v3.7.11 pins Yaegi v0.16.1 (`github.com/traefik/yaegi v0.16.1` is a direct require). Middleware plugins load through the local-plugin GOPATH interpreter, not compiled into Traefik.

Source: https://github.com/traefik/traefik/blob/v3.7.11/go.mod (blob SHA `bd36dbcfa5e58e59a277c8e346e01de02d259ed8`); extract `.sources/traefik-v3.7.11-go.mod.md`.

## Cross-package generics fail

A generic `Table[T]` **defined in another package** cannot be named at package scope in a plugin consumer (`*reclaim.Table[*BIN]` panics or fails import). Measured on Traefik v3.7.11 with compose local-plugin loader.

| Shape in consumer package | Result |
|---|---|
| Local `reclaim.NewTable[*T]()` inside `New` | Works |
| Package-level `var dbs *reclaim.Table[*T]` | Yaegi panic `nodeType2` |
| `var dbs any` then assert `*reclaim.Table[*T]` | Import error: type not found |

Source: [geoblock research folder](https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/knowledge/research/ext_traefik_plugins_yaegi-generics/notes.md) @ `22f09a0`; extract `.sources/compose-v3.7.11-workaround-matrix.md`.

## Workaround for a shared reclaim library

Use a **non-generic** table storing `any` in the shared package; callers type-assert in their own package. This matches geoblock `pkg/reclaim` (`Table` with `map[string]*slot`, `Open(..., create func() (any, error))`).

Do not ship `Table[T]` in `reclaim/` and instantiate it from middleware packages.

## Optional lifecycle interfaces under Yaegi

Type-switch on create's `any` is inert under Yaegi. Explicit `func()` hooks (a struct of funcs, extra Open parameters, or a named `Hooks` type in another interpreted package) do run.

This repo's `reclaim.Open` stores the create return and discovers `Sleep` / `Wake` / `Close` only via `sleepValue` / `wakeValue` / `closeValue` type-switches on that `any`. Those switches never match interpreted create returns.

Source: `reclaim/table.go` @ `c004ceb8d308cfbf6aaaa5957d63a4e42bd3e649`; extract `.sources/reclaim-table.go.md`.

Geoblock first recorded the method-set loss (same Yaegi pin). Source: `david-garcia-garcia/traefik-geoblock` @ `22f09a0` — `knowledge/debt/2026-09-08-yaegi-drops-methods-on-any.md`; extract `../ext_geoblock_pester-integration/.sources/2026-09-08-yaegi-drops-methods-on-any.md`.

The measurements below are a throwaway GOPATH interp (not a committed file): Yaegi v0.16.1, `interp.New(Options{GoPath: plugins-local})`, `Use(stdlib.Symbols)` only (no unsafe; matches this repo `docker-compose.yml` `useunsafe: false`). Interpreted package `hookprobe` imported this `reclaim` and a throwaway `hooklib`. Source: extract `.sources/yaegi-reclaim-hooks-probe.md`. Loader flags: `.sources/docker-compose.yml.md`.

### Create return: type-switch and method set are gone

Interpreted `create := func() (any, error) { return &life{}, nil }` then type-switch to same-package `sleeper` (`Sleep()`):

- `switch-sleeper=no`
- `%T` is `*struct { Xsleeps atomic.Int32; Xwakes atomic.Int32; Xcloses atomic.Int32 }`
- `reflect` method names empty
- comma-ok `value.(sleeper)`: `ok=false` (no panic on this return-through-any path)
- concrete `value.(*life)`: `ok=true` but `%T` still the synthesized struct

Source: `.sources/yaegi-reclaim-hooks-probe.md`.

### Same value passed into `func(any)` (not through create)

Type-switch to `sleeper` ran; no panic.

Source: `.sources/yaegi-reclaim-hooks-probe.md`. Geoblock's older probe saw comma-ok panic on the pass-into-any path; this run used type-switch and did not panic.

### Explicit `func()` hooks fire

These all fired (`sleeps=1 wakes=1 closes=1`):

- stored `func()` method values (`value.Sleep`)
- a same-package `hooks` struct of funcs
- extra Open-like `func()` parameters
- **cross-package** `hooklib.Hooks{Sleep: value.Sleep, ...}` plus `hooklib.DriveFuncs(...)`

Create signatures that return hooks next to `any` also worked:

- same-package `func() (any, hooks, error)`
- `hooklib.CreateWithHooks() (any, Hooks, error)` plus `DriveCreate(func() (any, hooklib.Hooks, error))` across interpreted packages

Source: `.sources/yaegi-reclaim-hooks-probe.md`.

### This repo's `reclaim.Open` under that interp

`reclaim.NewTable(20ms)` + `tab.Open` from interpreted `hookprobe`, create returning `*life` with Sleep/Wake/Close:

- logs printed: `reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_dispose`
- `life.sleeps=0 wakes=0 closes=0`

Table lifecycle logs run; optional methods on the stored value do not.

Source: `.sources/yaegi-reclaim-hooks-probe.md` (interp); `.sources/reclaim-table.go.md` (the type-switches those logs sit beside).

### Compiled `go test` does not catch this

Compiled `go test ./reclaim/` in the worktree: pass (1.075s). Contrast only; not a Yaegi fact.

Source: `.sources/yaegi-reclaim-hooks-probe.md`.

## E2e proof pattern

Geoblock catches Yaegi-only failures via Pester integration tests (docker Traefik + local plugin). A throwaway module outside the product tree evaluating packages through `interp` + `stdlib` mirrors this repo's loader (`useunsafe: false`; do not add `unsafe` to match geoblock's `useunsafe=true`). This spin-off needs an equivalent harness with a **fake middleware** that imports `reclaim/`.

Source: `david-garcia-garcia/traefik-geoblock` @ `22f09a0` — `devstate/2026/09/2026-09-08-reclaim-lifecycle/yaegi.md`, `scripts/integration-tests.Tests.ps1`; this repo `.sources/docker-compose.yml.md` and `.sources/yaegi-reclaim-hooks-probe.md`.
