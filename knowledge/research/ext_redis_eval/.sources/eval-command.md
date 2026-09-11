---
url: https://redis.io/docs/latest/commands/eval/
title: EVAL
fetched: 2026-09-11
authority: official
---

Syntax: EVAL script numkeys [key [key ...]] [arg [arg ...]]

Executes a server-side Lua script with the embedded Redis Lua 5.1 interpreter.

Required: script (Lua source), numkeys (number of key names that follow).

Optional keys populate KEYS; optional args populate ARGV. There must be exactly numkeys key arguments.

Example: EVAL "return ARGV[1]" 0 hello → "hello".

All keys that a script accesses must be explicitly provided as input key arguments.

Return information: depends on the script (see Lua API type conversion).
