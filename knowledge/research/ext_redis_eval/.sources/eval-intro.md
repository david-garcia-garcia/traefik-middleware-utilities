---
url: https://redis.io/docs/latest/develop/programmability/eval-intro/
title: Scripting with Lua
fetched: 2026-09-11
authority: official
---

Redis supports a single scripting engine, the Lua 5.1 interpreter.

EVAL second argument is the number of key-name arguments; value 0 when no keys.

KEYS table is pre-populated with key-name arguments; ARGV with regular arguments.

Example: EVAL "return { KEYS[1], KEYS[2], ARGV[1], ARGV[2], ARGV[3] }" 2 key1 key2 arg1 arg2 arg3 → array of five elements.

Lua table arrays returned from scripts are RESP2 array replies.

redis.call() errors are returned directly to the client; redis.pcall() returns errors to the script context.
