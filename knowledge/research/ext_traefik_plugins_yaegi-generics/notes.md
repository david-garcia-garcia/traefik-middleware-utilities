# Yaegi plugin generics and reclaim table shape

Traefik v3.7.11 pins Yaegi v0.16.1. Middleware plugins load through the local-plugin GOPATH interpreter, not compiled into Traefik.

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

Values returned through interpreted `func() (any, error)` lose method sets for interface assertions (`sleeper`, `waker`, `closer`). Type switches avoid comma-ok panics but optional hooks stay inert interpreted unless the API changes. Compiled `go test` does not catch this.

Source: `david-garcia-garcia/traefik-geoblock` @ `22f09a0` — `knowledge/debt/2026-09-08-yaegi-drops-methods-on-any.md`, `pkg/reclaim/table.go:134-143`.

## E2e proof pattern

Geoblock catches Yaegi-only failures via Pester integration tests (docker Traefik + local plugin). A throwaway module outside the product tree evaluating packages through `interp` + `stdlib` + `unsafe` mirrors Traefik's loader. This spin-off needs an equivalent harness with a **fake middleware** that imports `reclaim/`.

Source: `david-garcia-garcia/traefik-geoblock` @ `22f09a0` — `devstate/2026/09/2026-09-08-reclaim-lifecycle/yaegi.md`, `scripts/integration-tests.Tests.ps1`.
