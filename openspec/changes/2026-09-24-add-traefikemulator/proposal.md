## Why

Traefik middleware authors already copy a local `traefikemulator` test helper from the CrowdSec bouncer repo. This module has no published equivalent on `master`, so every consumer reimplements the same RouterFactory stand-in. Upstreaming the helper lets plugin tests depend on one module path beside `reclaim` and `iplookup`.

## What Changes

- New top-level package `traefikemulator/` (stdlib only): `Emulator`, `New`, `Apply`, `Stop`, `Handler`, `Serve`, `Route`, and `Constructor`.
- Port `emulator.go` and the six behavioral unit tests from `crowdsec-bouncer-traefik-plugin/pkg/traefikemulator` (rename `zzz_emulator_test.go` → `emulator_test.go`).
- Add focused unit tests until statement coverage matches the `iplookup` band (~95%+): duplicate route name, missing `Serve`/`Handler`, `Stop` lifecycle, optional empty `Apply`, optional `New(nil)` panic.
- No Yaegi, Go E2E, or Pester jobs for this package (compiled unit tests only; existing CI `test` and `race` jobs pick it up).
- README **Layout** row for the new package. Usage packet deferred to devdocsimpact.

## Capabilities

### New Capabilities

- `std_go_traefikemulator_generation`: one generation at a time; shared context per `Apply`; cancel previous generation before the next; partial constructor failure; route lookup and `Serve` semantics.

### Modified Capabilities

- None. `std_go_ci_test-suites` is unchanged: no new LIVE env, e2e, or Pester slice.

## Impact

- New `traefikemulator/` at module root.
- `README.md` Layout row.
- `openspec/specs/map.md` gains a `traefikemulator` family after archive.
- No changes to `reclaim/`, `iplookup/`, `simpleredis/`, CI job matrix, or the bouncer repo import path.
