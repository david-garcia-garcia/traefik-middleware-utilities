---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/docker-compose.yml
title: docker-compose reclaim routes (excerpt)
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-geoblock@22f09a0:docker-compose.yml
---

`whoami-reclaim`, `whoami-reclaim-b`, `whoami-reclaim-c`:
- Three routers, one middleware definition `geoblock-reclaim`.
- Middleware plugin labels under `traefik.http.middlewares.geoblock-reclaim.plugin.geoblock.*`.
- `blockedCountries=${RECLAIM_BLOCKED:-US}`, `allowedCountries=${RECLAIM_ALLOWED:-DE}` on primary service only.
- Pester recreates `whoami-reclaim` to change env → new Docker labels → Traefik dynamic reload.

Traefik service: see ext_traefik_plugins_local-loader docker-compose extract.
