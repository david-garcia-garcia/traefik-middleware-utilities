# ext / dragonfly

## EVAL
priority: normal
local: ext_dragonfly_eval/
description: Dragonfly EVAL undeclared-key enforcement, Lua 5.4 vs Redis 5.1, and script error wire format.

## Container image
priority: normal
local: ext_dragonfly_container-image/
description: Official Dragonfly Docker image registry, tag pin, port 6379, redis-cli, and compose gotchas for CI.

## BLPOP
priority: normal
local: ext_dragonfly_blpop/
description: Dragonfly BLPOP command-surface support and empty-list connection block for live I/O timeout proof.
