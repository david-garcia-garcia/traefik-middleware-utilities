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
description: Whether Yaegi v0.16.1 stdlib.Symbols exports context.AfterFunc for interpreted reclaim.
