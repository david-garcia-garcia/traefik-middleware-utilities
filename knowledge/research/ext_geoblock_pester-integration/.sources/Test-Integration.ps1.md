---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/Test-Integration.ps1
title: Test-Integration.ps1
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-geoblock@22f09a0:Test-Integration.ps1
---

Local integration runner (superset of CI):
- Ensures Pester installed.
- On Windows: switch Docker Desktop to Linux engine via DockerCli.exe.
- Loads repo `.env` into process env (keys only logged).
- `docker compose up -d` or `docker compose --profile local-tokens up -d` if IP2LOCATION_DOWNLOAD_TOKEN set.
- Waits for Traefik API + /foo + /bar (same URLs as CI).
- Runs Pester on `./scripts` (integration-tests.Tests.ps1).
- Optional `-SkipDockerCleanup` leaves stack up; default runs `docker compose down -v`.

Params: `-SkipWait` (assume stack running), `-TestPath` (default `./scripts`).
