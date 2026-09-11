---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/scripts/integration-tests.Tests.ps1
title: integration-tests.Tests.ps1 (harness surface)
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-geoblock@22f09a0:scripts/integration-tests.Tests.ps1
---

BeforeDiscovery: load `.env`, set skip flags for token-gated tests.

BeforeAll helpers:
- `Wait-TraefikPluginLog` — regex against `docker logs traefik`
- `Invoke-TestRequest` — GET with X-Real-IP, returns status/content
- `Get-TraefikAccessLogEntries` — `docker exec traefik tail` JSON access log
- `Get-ReclaimLogEvents` — parse stdout for reclaim_* log lines
- `Wait-ReclaimPluginPutCount`, `Wait-ReclaimDispose` — reclaim timing

Config:
- `$script:BaseUrl = "http://localhost:8000"`
- `$script:TraefikApiUrl = "http://localhost:8080"`

Tests assume stack already running (started by CI or Test-Integration.ps1 before Invoke-Pester).
