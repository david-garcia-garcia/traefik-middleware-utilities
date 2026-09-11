---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/knowledge/devdocs/core_geoblock_test-harness.md
title: core_geoblock_test-harness (integration add pattern)
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-geoblock@22f09a0:knowledge/devdocs/core_geoblock_test-harness.md
---

To test Traefik-visible behavior:
1. Add `whoami-*` in docker-compose with unique PathPrefix + plugin labels.
2. Add Pester Context/It in scripts/integration-tests.Tests.ps1.

Named examples:
- Shared middleware + config grace: `/reclaima` `/reclaimb` `/reclaimc`
- Middleware chain: `/enrichthenblock`
- Block-only: `/blockonly`

CI: `.github/workflows/ci.yml` — lint, go test, integration job.

Do not call `go test` an integration test.
