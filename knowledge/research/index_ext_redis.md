# ext / redis

## INCR and INCRBY
priority: normal
local: ext_redis_incr/
description: Redis INCR/INCRBY wire replies, missing-key init, TTL preservation, and error cases.

## EXPIRE and EXPIREAT
priority: normal
local: ext_redis_expire/
description: Redis EXPIRE/EXPIREAT integer :0/:1 replies, missing keys, and delete-on-non-positive behavior.

## EVAL
priority: normal
local: ext_redis_eval/
description: Redis EVAL argument shape, zero keys, Lua-to-RESP reply mapping, and script error format.

## EVALSHA
priority: normal
local: ext_redis_evalsha/
description: Redis EVALSHA SHA-1 digest, volatile script cache, and NOSCRIPT miss wire text.

## Pipelining
priority: normal
local: ext_redis_pipelining/
description: Redis RESP pipelining: one write of N commands, N ordered replies, mixed verbs including EVAL, and -ERR vs I/O.

## RESP bulk strings
priority: normal
local: ext_redis_resp_bulk-string/
description: RESP2 bulk-string wire form, null bulk, and why a truncated payload is a transport fake not a Redis command.

## RESP2 null array
priority: normal
local: ext_redis_resp_null-array/
description: Official RESP2 encoding of a null array (*-1) versus null bulk and empty array.

## Idle client close
priority: normal
local: ext_redis_clients_idle-close/
description: How Redis closes a client TCP socket from the server (idle timeout vs CLIENT KILL).

## MSETEX
priority: normal
local: ext_redis_msetex/
description: Redis 8.4 MSETEX argv/reply and Redis 7 unknown-command detection for engines that lack it.
