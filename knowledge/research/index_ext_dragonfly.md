# ext / dragonfly

## EVAL
priority: normal
local: ext_dragonfly_eval/
description: Dragonfly EVAL undeclared-key enforcement, Lua 5.4 vs Redis 5.1 (table.maxn and unpack), and script error wire format.

## EVALSHA
priority: normal
local: ext_dragonfly_evalsha/
description: Dragonfly EVALSHA NOSCRIPT wire text on v1.40.2 and SCRIPT EXISTS/FLUSH for live proof.

## Container image
priority: normal
local: ext_dragonfly_container-image/
description: Official Dragonfly Docker image registry, tag pin, port 6379, redis-cli, and compose gotchas for CI.

## Pipelining
priority: normal
local: ext_dragonfly_pipelining/
description: Dragonfly Redis-compat pipelining, default queue/buffer limits, and huge-pipeline deadlock flags for CI.

## MSETEX
priority: normal
local: ext_dragonfly_msetex/
description: Dragonfly v1.40 command matrix has no MSETEX; Lua SET EX/EXAT is the group-write path.
