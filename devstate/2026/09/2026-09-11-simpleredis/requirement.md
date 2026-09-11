# Requirement
IssueKey: 2026-09-11-simpleredis

## Problem
Dest has no Redis client library. Callers cannot import a shared SimpleRedis (or Redis connection) from `github.com/david-garcia-garcia/traefik-middleware-utilities`. README still lists Redis as Planned. The ticket wants the crowdsec-bouncer `pkg/simpleredis` sources and tests copied here, plus Yaegi tests and a Redis-using host plugin for Pester e2e.

## Current (code)
- `redis/` — not found on `origin/master` (`git ls-tree origin/master`; layout is only promised in `README.md`).
- `simpleredis/` — not found on `origin/master`.
- `README.md` Libraries table: Redis connection is Planned; Layout block lists `redis/` as later. Reclaim table is Current (`reclaim/`).
- `go.mod` — module `github.com/david-garcia-garcia/traefik-middleware-utilities`, Go 1.21, require only `github.com/traefik/yaegi v0.16.1`. No Redis client module.
- `reclaim/yaegi_test.go` — GOPATH Yaegi interp (`stdlib` only, `useunsafe` false) proving reclaim under the interpreter.
- `e2e/reclaimprobe/` — fake Traefik plugin (`Config`, `CreateConfig`, `New`) that imports `reclaim`. Manifest `e2e/reclaimprobe/.traefik.yml`. Module `github.com/david-garcia-garcia/reclaimprobe` with replace to repo root.
- `docker-compose.yml` — `traefik:v3.7.11`, local plugin `reclaimprobe`, whoami `/a` and `/b`. No Redis service.
- `Test-Integration.ps1` — compose up, wait Traefik API + `/a` `/b`, Invoke-Pester `scripts/integration-tests.Tests.ps1`.
- `scripts/integration-tests.Tests.ps1` — reclaim Yaegi e2e only (shared incarnation, sleep/wake/close logs).
- `.github/workflows/ci.yml` — golangci-lint, `go test -v ./...`, `./Test-Integration.ps1`.
- Root `LICENSE` — not found on dest.
- Third-party source (research, not dest): `pkg/simpleredis` in `david-garcia-garcia/crowdsec-bouncer-traefik-plugin` @ `6548da47` — stdlib RESP client (`SimpleRedis.Init/Get/MGet/Set/Del/Close`), tests use in-process fake TCP Redis (`simpleredis_test.go`), Apache-2.0 repo LICENSE. Facts: `knowledge/research/ext_crowdsec_simpleredis/notes.md`.

## Desired
1. Copy existing simpleredis sources and their existing test coverage from that repo into this module so other projects import them from `github.com/david-garcia-garcia/traefik-middleware-utilities`.
2. Besides those tests, add Yaegi-specific tests (README Yaegi rules; dest pattern `reclaim/yaegi_test.go`).
3. Create a host plugin that uses Redis and wrap-up Pester e2e following dest reclaim: `e2e/`, docker-compose, `Test-Integration.ps1`, `scripts/integration-tests.Tests.ps1`.
4. Treat this as the next product library (README Planned Redis connection; layout `redis/`).

## Affected
- New library tree (`redis/` per README layout, or `simpleredis/` if the package name is kept — tension).
- `README.md` Libraries/Layout/Tests.
- Yaegi tests next to that library (dest: `reclaim/yaegi_test.go`).
- New `e2e/` host plugin (dest: `e2e/reclaimprobe/`).
- `docker-compose.yml` (Redis service and/or second local plugin — not present now).
- `Test-Integration.ps1`, `scripts/integration-tests.Tests.ps1`, `.github/workflows/ci.yml`.
- Possibly root license/NOTICE for the Apache-2.0 copy (root LICENSE not found).

## Out of scope
- Copying crowdsec `pkg/cache`, LAPI, captcha, or decisionscope.
- Adding `go-redis` / miniredis / TLS / Unix sockets (source client is stdlib TCP RESP only).
- Changing reclaim behavior or reclaim e2e semantics except as needed to share the harness.
- Publishing a GitHub release / new module path.

## Unknowns
- Folder and import last segment: README `redis/` vs source package `simpleredis`.
- Whether the host plugin shares the existing reclaim compose project or gets its own stack/ports.
- Whether e2e Redis is a compose `redis` container or a fake like `startFakeRedis` in the source tests.
- How Apache-2.0 attribution lands given dest has no root LICENSE.
- Whether `Test-Integration.ps1` stays one script for both libraries or splits.

## Tensions
- Ticket names the package simpleredis; dest README names the Planned library “Redis connection” under `redis/`.
- Ticket asks to copy existing tests (in-process fake Redis, no live server) and also a host plugin that uses Redis for Pester (likely a real Redis in compose — dest reclaim e2e has no Redis).
- Ticket “host plugin that uses Redis” vs dest e2e plugin that only imports the in-repo library; source SimpleRedis is not itself a Traefik plugin (`plugin.go` does not import it).
