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

## RESP2 null array
priority: normal
local: ext_redis_resp_null-array/
description: Official RESP2 encoding of a null array (*-1) versus null bulk and empty array.
