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

## AUTH
priority: normal
local: ext_redis_auth/
description: Redis AUTH requirepass/ACL replies (WRONGPASS, nopass AUTH text) on redis:7-alpine.

## SELECT
priority: normal
local: ext_redis_select/
description: Redis SELECT logical-DB indexes and the DB-index-out-of-range error.
