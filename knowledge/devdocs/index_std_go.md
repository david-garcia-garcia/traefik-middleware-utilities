# std / go

## Reclaim table
priority: normal
local: std_go_reclaim.md
description: How a Traefik middleware stores one value per key with create, sleep, wake, and close.

## SimpleRedis
priority: normal
local: std_go_simpleredis.md
description: How a Traefik middleware builds SimpleRedis with New(Config) and speaks GET/MGET/SET/DEL/INCR/EXPIRE/EVAL/MSetEX/MSetEXAt and ExecPipeline over stdlib TCP RESP.

## RESP decode
priority: normal
local: std_go_simpleredis_resp-decode.md
description: How SimpleRedis reads RESP lines with ReadSlice, copies escaping +/: payloads, and parses lengths from bytes.

## Window counter
priority: normal
local: std_go_windowcounter.md
description: How a Traefik middleware Takes a sliding-window hit against Redis or Dragonfly via SimpleRedis.

## Token bucket
priority: normal
local: std_go_tokenbucket.md
description: How a Traefik middleware Allows a Traefik RateLimit token-bucket consume in process or via SimpleRedis Eval.

