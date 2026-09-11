---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/docker-compose.yml
title: docker-compose.yml (geoblock local plugin)
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-geoblock@22f09a0:docker-compose.yml
---

Traefik service:
- `image: traefik:v3.7.11`
- Static flags:
  - `--experimental.localplugins.geoblock.modulename=github.com/david-garcia-garcia/traefik-geoblock`
  - `--experimental.localplugins.geoblock.settings.useunsafe=true`
- Volume mount (local plugin GOPATH):
  - `./:/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`
- Also mounts docker.sock, access log volume, seed HTML.
- Ports: `8000:80` (HTTP entrypoint), `8080:8080` (API insecure).

Dynamic middleware labels use alias `geoblock`:
- `traefik.http.middlewares.geoblock2.plugin.geoblock.*=...`

Paths inside container reference mount prefix `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock/...`.
