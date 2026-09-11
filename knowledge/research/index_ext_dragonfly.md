# ext / dragonfly

## EVAL
priority: normal
local: ext_dragonfly_eval/
description: Dragonfly EVAL undeclared-key enforcement, Lua 5.4 vs Redis 5.1, and script error wire format.

## Container image
priority: normal
local: ext_dragonfly_container-image/
description: Official Dragonfly Docker image registry, tag pin, port 6379, redis-cli, and compose gotchas for CI.

## AUTH
priority: normal
local: ext_dragonfly_auth/
description: Dragonfly AUTH with and without --requirepass, including nopass-accepts-any-password.

## SELECT
priority: normal
local: ext_dragonfly_select/
description: Dragonfly SELECT --dbnum default and the DB-index-out-of-range error.
