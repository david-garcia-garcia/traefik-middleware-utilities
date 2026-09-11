## Why

The reclaim table that Traefik middlewares use to keep one value per key (create → sleep/wake cycles → close) lives only inside `traefik-geoblock`. This repo exists so that table can be imported instead of copied, and so Yaegi-only failures are caught by a fake middleware under Traefik — compiled `go test` cannot see them.

## What Changes

- Add module `github.com/david-garcia-garcia/traefik-middleware-utilities` and package `reclaim/` ported from geoblock PR #83 (`pkg/reclaim` @ `22f09a0`): non-generic `Table` of `any`, `Open`, `Default`, `NewTable`, optional `Sleep`/`Wake`/`Close`.
- Port unit tests from `table_test.go`.
- Add a nested fake Traefik plugin (`e2e/reclaimprobe`) and Pester+Docker e2e that loads it through Yaegi (`traefik:v3.7.11`).
- Bring in reclaim specs and the `std_go_reclaim` usage packet (paths adapted from `pkg/reclaim` to `reclaim/`).
- Commit the library README (layout and Yaegi rules).

## Capabilities

### New Capabilities

- `std_go_reclaim_context-lease`: Keyed table, Open/create-once, context holders, grace, logging, Yaegi-safe `create func() (any, error)`, loadable as an import from a Traefik local plugin.
- `std_go_reclaim_value-lifecycle`: Four events create → (sleep → wake)* → sleep → close; optional interfaces; caller never receives a sleeping value.

### Modified Capabilities

- None. Catalog is empty on DestBranch.

## Impact

- New `go.mod`, `reclaim/` (stdlib only), `README.md`.
- New `e2e/reclaimprobe/` (Traefik `CreateConfig`/`New`), `docker-compose.yml`, `Test-Integration.ps1`, `scripts/integration-tests.Tests.ps1`.
- New `openspec/specs/std_go_reclaim_*`.
- Consumers later replace `traefik-geoblock/pkg/reclaim` imports; no geoblock or Redis/leaky-bucket work in this change.
- `Open` signature is unchanged. Optional lifecycle hooks stay inert under Yaegi (documented debt).
