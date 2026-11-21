# std / go

## Reclaim table
priority: normal
local: std_go_reclaim.md
description: How a Traefik middleware stores one value per key with create, sleep, wake, and close.

## SimpleRedis
priority: normal
local: std_go_simpleredis.md
description: How a Traefik middleware Inits and speaks GET/MGET/SET/DEL/INCR/EXPIRE/EVAL/MSetEX/MSetEXAt over stdlib TCP RESP.

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

## Backend backoff
priority: normal
local: std_go_backendbackoff.md
description: How a Traefik middleware Allows then Reports a per-key in-memory backoff gate for an unhealthy backend.

## CIDR lookup
priority: normal
local: std_go_iplookup.md
description: How a Traefik middleware stores CIDR prefixes by address family and returns the winning match label.

## Test suites
priority: normal
local: std_go_test-suites.md
description: Lint, unit Go (plain `-short` and `Unit race`), Go E2E Redis, Go E2E Dragonfly, and Pester (reclaim, Redis, Dragonfly) — what each suite is for and how test files are named.

