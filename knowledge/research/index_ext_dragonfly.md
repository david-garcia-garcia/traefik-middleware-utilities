# ext / dragonfly

## EVAL
priority: normal
local: ext_dragonfly_eval/
description: Dragonfly EVAL undeclared-key enforcement, Lua 5.4 vs Redis 5.1, and script error wire format.

## Container image
priority: normal
local: ext_dragonfly_container-image/
description: Official Dragonfly Docker image registry, tag pin, port 6379, redis-cli, and compose gotchas for CI.

## Idle client close
priority: normal
local: ext_dragonfly_clients_idle-close/
description: How Dragonfly closes a client TCP socket from the server (idle --timeout vs CLIENT KILL ADDR/ID).
