# ext / traefik

## Local plugin loader
priority: normal
local: ext_traefik_plugins_local-loader/
description: How Traefik loads a local middleware plugin via Yaegi GOPATH layout and static config.

## Yaegi plugin generics
priority: normal
local: ext_traefik_plugins_yaegi-generics/
description: Which generic and cross-package type shapes load under Traefik's Yaegi interpreter.

## RateLimit token bucket
priority: normal
local: ext_traefik_ratelimiter_token-bucket/
description: Traefik RateLimit token-bucket math, in-memory vs Redis Lua, and GCRA-not-in-tree vs PR 10211.
