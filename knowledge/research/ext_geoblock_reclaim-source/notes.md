# Geoblock PR #83 reclaim table — spin-off source

Source repo: `https://github.com/david-garcia-garcia/traefik-geoblock` PR #83, HEAD `22f09a059a2f0c59e7669b017e69951059903912` (temp clone, read-only).

## Package to port

| Path | Role |
|---|---|
| `pkg/reclaim/table.go` | `Table`, four-state lifecycle (create → sleep/wake cycles → close), stdlib-only imports |
| `pkg/reclaim/default.go` | Process-wide `Default()`, package `Open`, test `Reset` |
| `pkg/reclaim/table_test.go` | Extensive unit tests (concurrency, grace, lifecycle ordering, Yaegi-shaped nil-Done context) |

Target layout in this repo: `reclaim/` (not `pkg/reclaim/`) per destination README intent.

## Specs to bring in (reclaim-only)

| Spec id | Purpose |
|---|---|
| `std_go_reclaim_context-lease` | Keyed table, Open/create-once, context holders, grace, logging, Yaegi constraints (`create func() (any, error)`, no generics) |
| `std_go_reclaim_value-lifecycle` | Four events: create, sleep, wake, close; optional interfaces; caller never receives sleeping value |

Skip geoblock-specific specs (`core_geoblock_database_wrapper-reclaim`, `core_geoblock_plugin_instance-reclaim`, etc.).

## Related knowledge in source (adapt later)

- `knowledge/devdocs/std_go_reclaim.md` — usage packet for reclaim table
- `knowledge/research/ext_traefik_plugins_yaegi-generics/` — Yaegi type shapes (copied separately into this repo)

## Tests beyond unit

Geoblock PR #83 adds no reclaim-specific Pester cases; integration tests exercise the full geoblock plugin. This ticket **requires new** e2e coverage: fake middleware + Pester + Yaegi load path for `reclaim/` alone.

## Module note

Source module: `github.com/david-garcia-garcia/traefik-geoblock`. Destination module: `github.com/david-garcia-garcia/traefik-middleware-utilities` — import paths and any consumer references must change on port.
