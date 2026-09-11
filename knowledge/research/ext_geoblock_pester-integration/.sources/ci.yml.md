---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/.github/workflows/ci.yml
title: CI integration job
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-geoblock@22f09a0:.github/workflows/ci.yml
---

Integration job on `ubuntu-latest`:
1. Checkout, install PowerShell + Pester (`Install-Module Pester -Force -Scope CurrentUser`).
2. `docker compose up -d`
3. pwsh wait loop: Traefik API `:8080/api/rawdata`, then `:8000/foo`, then `:8000/bar` (60s timeout each phase).
4. `Invoke-Pester -Path ./scripts/integration-tests.Tests.ps1 -Output Detailed`
5. On failure: dump traefik + whoami logs.
6. Always: `docker compose down -v`

Separate jobs: lint (golangci-lint), unit test (`go test -v ./...`).
