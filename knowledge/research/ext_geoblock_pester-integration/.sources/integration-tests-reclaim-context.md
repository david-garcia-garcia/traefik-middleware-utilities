---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/scripts/integration-tests.Tests.ps1
title: Shared middleware incarnation Context (excerpt)
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-geoblock@22f09a0:scripts/integration-tests.Tests.ps1
---

Context "Shared middleware incarnation and config change":
- Routes `/reclaima`, `/reclaimb`, `/reclaimc` share middleware `geoblock-reclaim@docker`.
- Assert one `reclaim_put` for three routes (one plugin incarnation).
- Set `$env:RECLAIM_BLOCKED`, `$env:RECLAIM_ALLOWED`; `docker compose up -d --no-deps --force-recreate whoami-reclaim`.
- Expect second `reclaim_put` with new plugin key hash.
- Assert old key `reclaim_dispose` not immediate; after ~10s grace, dispose fires.
- Probe HTTP: US allowed after config flip.

Uses compose env interpolation on whoami-reclaim labels (`${RECLAIM_BLOCKED:-US}`).
