# ext / traefik

## Local plugin loader
priority: normal
local: ext_traefik_plugins_local-loader/
description: How Traefik loads a local middleware plugin via Yaegi GOPATH layout and static config.

## Yaegi plugin generics
priority: normal
local: ext_traefik_plugins_yaegi-generics/
description: Which generic and cross-package type shapes load under Traefik's Yaegi interpreter.

## Yaegi context.AfterFunc
priority: normal
local: ext_traefik_plugins_yaegi-afterfunc/
description: How Yaegi v0.16.1 maps and runs context.AfterFunc for interpreted reclaim.

## RateLimit token bucket
priority: normal
local: ext_traefik_ratelimiter_token-bucket/
description: Traefik RateLimit token-bucket math, in-memory vs Redis Lua, and GCRA-not-in-tree vs PR 10211.

## CircuitBreaker middleware
priority: normal
local: ext_traefik_circuitbreaker/
description: Traefik CircuitBreaker expression examples, duration defaults, and Closed/Open/Recovering — not a library this product imports.

## Plugin useUnsafe load gate
priority: normal
local: ext_traefik_plugins_useunsafe/
description: How Traefik v3 Yaegi plugins register unsafe/syscall only when both the manifest and operator settings set useUnsafe.

## Yaegi unsafe symbols
priority: normal
local: ext_traefik_plugins_yaegi-unsafe/
description: Which unsafe package symbols Yaegi v0.16.1 exports when Traefik registers stdlib/unsafe.

## Yaegi build constraints
priority: normal
local: ext_traefik_plugins_yaegi-build-constraints/
description: Which build-constraint mechanisms Yaegi v0.16.1 honors when assembling an interpreted package, and which it ignores silently.
