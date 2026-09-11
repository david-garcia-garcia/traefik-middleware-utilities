## 1. Module skeleton

- [x] 1.1 Add `go.mod` (`github.com/david-garcia-garcia/traefik-middleware-utilities`, Go 1.21) and commit the library `README.md` (reclaim layout + Yaegi rules)
- [x] 1.2 Add `openspec/specs/domains.md` (`std` / `go`) if missing from DestBranch

## 2. Reclaim package

- [x] 2.1 Port `table.go` and `default.go` from geoblock PR #83 @ `22f09a0` `pkg/reclaim` into `reclaim/` (stdlib imports only; type-switch optional interfaces)
- [x] 2.2 Port `table_test.go` and adapt import paths
- [x] 2.3 Run `go test ./reclaim/...` on this host until passing

## 3. Yaegi e2e harness

- [x] 3.1 Add `e2e/reclaimprobe/` fake plugin (`package reclaimprobe`, `Config`, `CreateConfig`, `New` calling `reclaim.Open`, passthrough handler + identity headers, slog to stdout)
- [x] 3.2 Add `.traefik.yml` and compose: `traefik:v3.7.11`, whoami, dual GOPATH mounts, `useunsafe: false`, two routes sharing one probe middleware
- [x] 3.3 Add `Test-Integration.ps1` and `scripts/integration-tests.Tests.ps1` (API wait, request both routes, shared value identity, `reclaim_put`/`reclaim_bind` in Traefik logs)
- [x] 3.4 Run the Pester e2e on this host with Docker until passing

## 4. Specs on disk for apply

- [x] 4.1 Confirm change specs `std_go_reclaim_context-lease` and `std_go_reclaim_value-lifecycle` match the ported API (no `pkg/reclaim` paths)
- [x] 4.2 Run `openspec validate --change add-reclaim-table --strict`
