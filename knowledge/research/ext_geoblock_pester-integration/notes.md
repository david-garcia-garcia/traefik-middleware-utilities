# Geoblock Pester + Docker Traefik integration harness

How geoblock @ `22f09a0` starts Traefik with a local Yaegi plugin and runs Pester e2e tests. Pattern to copy for a **fake middleware** in this repo that imports `reclaim/` under the same loader path.

## Startup flow

| Step | Owner | Action |
|------|-------|--------|
| CI / local | `.github/workflows/ci.yml` or `Test-Integration.ps1` | `docker compose up -d` from repo root |
| Wait | same | Poll `http://localhost:8080/api/rawdata` (Traefik API), then `http://localhost:8000/foo` and `/bar` (whoami through plugin) |
| Test | `scripts/integration-tests.Tests.ps1` | `Invoke-Pester -Path ./scripts/integration-tests.Tests.ps1` |
| Teardown | CI always; local optional | `docker compose down -v` |

Local runner (`Test-Integration.ps1`) additionally switches Docker Desktop to Linux containers on Windows, loads `.env` for compose interpolation, and optionally enables compose profile `local-tokens` when `IP2LOCATION_DOWNLOAD_TOKEN` is set.

Extracts: [.sources/ci.yml.md](.sources/ci.yml.md), [.sources/Test-Integration.ps1.md](.sources/Test-Integration.ps1.md)

## Compose stack essentials

- **Traefik**: `traefik:v3.7.11`, local plugin mount + static `localPlugins` (see [ext_traefik_plugins_local-loader/](../ext_traefik_plugins_local-loader/notes.md)).
- **Backends**: `traefik/whoami` services with Docker labels defining routers, middlewares, and plugin dynamic config.
- **Ports**: `8000` → web entrypoint, `8080` → API (`--api.insecure=true`).

Tests use `$script:BaseUrl = "http://localhost:8000"` and `$script:TraefikApiUrl = "http://localhost:8080"`.

Extracts: [.sources/docker-compose.yml.md](.sources/docker-compose.yml.md), [.sources/integration-tests.Tests.ps1.md](.sources/integration-tests.Tests.ps1.md)

## Adding a route + middleware for a new behavior

Geoblock convention ([core_geoblock_test-harness](https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/knowledge/devdocs/core_geoblock_test-harness.md)):

1. Add a `whoami-*` service in `docker-compose.yml` with unique `PathPrefix`, router labels, and `traefik.http.middlewares.<name>.plugin.geoblock.*` settings.
2. Add a Pester `Context` / `It` in `scripts/integration-tests.Tests.ps1` hitting that path.

For reclaim lifecycle, geoblock uses `/reclaima`, `/reclaimb`, `/reclaimc` sharing one middleware `geoblock-reclaim@docker`. Pester recreates `whoami-reclaim` with new env (`RECLAIM_BLOCKED`, `RECLAIM_ALLOWED`) to force a dynamic config reload and second plugin incarnation.

Extracts: [.sources/core_geoblock_test-harness.md](.sources/core_geoblock_test-harness.md), [.sources/integration-tests-reclaim-context.md](.sources/integration-tests-reclaim-context.md)

## Observing reclaim under Traefik

Pester helpers parse Traefik stdout (not access log):

- `Get-ReclaimLogEvents` — lines matching `reclaim_put|dispose|bind|orphan|reclaim`
- `Wait-ReclaimPluginPutCount`, `Wait-ReclaimDispose` — timing assertions on grace (~10s default)

These depend on geoblock plugin logging at DEBUG and middleware `logLevel=DEBUG`. A minimal fake middleware should emit the same structured log keys if reusing these helpers.

Extracts: [.sources/integration-tests.Tests.ps1.md](.sources/integration-tests.Tests.ps1.md)

## Yaegi-only failures: two guards

Geoblock uses **two** layers because `go test` compiles and misses interpreter-only bugs:

1. **Pester + docker Traefik** — full process; catches panics on background goroutines that kill Traefik (symptom: API check fails). Example: multi-assignment with untyped `nil` in `pkg/dbsource/updater.go` ([yaegi.md](https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/devstate/2026/09/2026-09-08-reclaim-lifecycle/yaegi.md)).
2. **Scratch probe module** outside the plugin tree — `interp.New` with `GoPath` = `plugins-local`, `Use(stdlib.Symbols)` + `Use(unsafe.Symbols)`, evaluate real packages without starting Traefik. Geoblock used `D:/repositories/scratch-yaegi-geoblock` @ Yaegi v0.16.1.

Optional-interface method loss on `func() (any, error)` returns is documented in [ext_traefik_plugins_yaegi-generics/](../ext_traefik_plugins_yaegi-generics/notes.md) — not duplicated here.

Extracts: [.sources/yaegi.md.md](.sources/yaegi.md.md), [.sources/2026-09-08-yaegi-drops-methods-on-any.md](.sources/2026-09-08-yaegi-drops-methods-on-any.md)

## Spin-off: fake middleware importing `reclaim/`

Geoblock PR #83 has **no** reclaim-only Pester cases; reclaim is exercised through the full geoblock plugin. This ticket needs a **minimal** plugin in `traefik-middleware-utilities`:

| Piece | Geoblock pattern | Spin-off |
|-------|------------------|----------|
| Module | `github.com/david-garcia-garcia/traefik-geoblock` | `github.com/david-garcia-garcia/traefik-middleware-utilities` |
| Mount | `./:/plugins-local/src/.../traefik-geoblock` | `./:/plugins-local/src/.../traefik-middleware-utilities` |
| Root package | `traefik_geoblock` | `traefik_middleware_utilities` |
| Subpackage under test | `pkg/reclaim` | `reclaim/` |
| Static alias | `geoblock` | e.g. `reclaimtest` |
| Fake `New` | calls `reclaim.Open`, returns passthrough handler | same shape, minimal config |
| Compose service | `whoami-reclaim` + shared routes | new `whoami-*` + Pester context |

Keep `go.mod` / `vendor/` out of Yaegi dependency fetch: fake middleware should stay stdlib + in-repo `reclaim/` only for first e2e slice; add vendor only when third-party deps appear.

Extracts: [.sources/ext_geoblock_reclaim-source-notes.md](.sources/ext_geoblock_reclaim-source-notes.md)

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| CI: compose up → wait → Pester → down | geoblock@22f09a0:ci.yml | source |
| Local runner steps + Linux container switch | geoblock@22f09a0:Test-Integration.ps1 | source |
| Base URLs and health checks | geoblock@22f09a0:integration-tests.Tests.ps1 | source |
| Reclaim log parsing helpers | geoblock@22f09a0:integration-tests.Tests.ps1 | source |
| Add whoami + Pester for new behavior | geoblock@22f09a0:core_geoblock_test-harness.md | source |
| Scratch Yaegi probe outside module | geoblock@22f09a0:yaegi.md | ticket |
| Optional-interface Yaegi limitation | geoblock@22f09a0:2026-09-08-yaegi-drops-methods-on-any.md | ticket |
| No reclaim-only Pester in PR #83 | geoblock@22f09a0 (this repo ext_geoblock_reclaim-source) | inference |

## References

- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/.github/workflows/ci.yml
- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/Test-Integration.ps1
- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/scripts/integration-tests.Tests.ps1
- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/docker-compose.yml
- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/knowledge/devdocs/core_geoblock_test-harness.md
- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/devstate/2026/09/2026-09-08-reclaim-lifecycle/yaegi.md
- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/knowledge/debt/2026-09-08-yaegi-drops-methods-on-any.md
