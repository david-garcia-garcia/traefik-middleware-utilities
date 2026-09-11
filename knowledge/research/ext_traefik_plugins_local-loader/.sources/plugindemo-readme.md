---
url: https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md
title: Developing a Traefik plugin (plugindemo)
fetched: 2026-09-11
authority: official
ref: github.com/traefik/plugindemo@44419f66fe21c51f4c94fd46f8e02b98e4fb3168:readme.md
---

Middleware plugin = Go package exporting `http.Handler` via Yaegi (not pre-compiled).

Required exports:
- `type Config struct`
- `func CreateConfig() *Config`
- `func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)`

`.traefik.yml` `import`: Go import path of the plugin package.

Dependencies must be vendored; Go modules not supported for plugin deps.

Local mode: GOPATH workspace `./plugins-local/src/<module path>/`.

Static config: `experimental.localPlugins.<alias>.moduleName` = that module path.

Plugins parsed and loaded only at Traefik startup; dynamic config drives instantiation afterward.
