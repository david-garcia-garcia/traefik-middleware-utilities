---
url: https://redis.io/docs/latest/develop/programmability/lua-api/
title: Redis Lua API — Lua to RESP2 type conversion
fetched: 2026-09-11
authority: official
---

Default protocol during script execution: RESP2.

Lua to RESP2:
- Lua number → RESP2 integer reply (decimal part removed)
- Lua string → RESP2 bulk string reply
- Lua indexed array table → RESP2 array reply (truncated at first nil)
- Lua table with single ok field → RESP2 status reply
- Lua table with single err field → RESP2 error reply
- Lua boolean false → RESP2 null bulk reply
- Lua boolean true → RESP2 integer reply with value 1

Examples:
- EVAL "return 10" 0 → (integer) 10
- EVAL "return { 1, 2, { 3, 'Hello World!' } }" 0 → nested array
- EVAL "return redis.call('get','foo')" 0 → bulk string when GET returns a string

redis.call() runtime errors: returned directly to client as error reply (example shows ERR Wrong number of args calling Redis command from script …).
