---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/plugin.go
title: plugin.go (geoblock Traefik entry)
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-geoblock@22f09a0:plugin.go
---

`package traefik_geoblock` — matches Traefik basePkg rule for module `.../traefik-geoblock`.

Exports:
- `type Config = geoblock.Config` (alias)
- `func CreateConfig() *Config` → delegates to `geoblock.CreateConfig()`
- `func New(ctx, next, cfg, name) (http.Handler, error)` → `geoblock.Prepare`, then `reclaim.Open` for plugin instance reuse

Imports subpackages:
- `github.com/david-garcia-garcia/traefik-geoblock/pkg/geoblock`
- `github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim`

Pattern: Traefik-required symbols on root; business logic in `pkg/…` loaded via GOPATH when root imports them.
