---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/c004ceb8d308cfbf6aaaa5957d63a4e42bd3e649/docker-compose.yml
title: docker-compose.yml local plugin Yaegi flags
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-middleware-utilities@c004ceb8d308cfbf6aaaa5957d63a4e42bd3e649:docker-compose.yml
---

Traefik image: `traefik:v3.7.11`.

Local plugin flag that matches the 2026-09-11 GOPATH interp (stdlib only, no unsafe):

`--experimental.localplugins.reclaimprobe.settings.useunsafe=false`

GOPATH mounts:

- `./e2e/reclaimprobe` → `/plugins-local/src/github.com/david-garcia-garcia/reclaimprobe`
- `./` → `/plugins-local/src/github.com/david-garcia-garcia/traefik-middleware-utilities`
