---
url: https://redis.io/docs/latest/commands/evalsha/
title: EVALSHA
fetched: 2026-09-11
authority: official
---

Syntax: EVALSHA sha1 numkeys [key [key ...]] [arg [arg ...]]

Executes a server-side Lua script by SHA1 digest. Otherwise identical to EVAL.

Required: sha1 (digest of a script previously cached with SCRIPT LOAD or EVAL), numkeys.

Optional keys populate KEYS; optional args populate ARGV. There must be exactly numkeys key arguments.

Return depends on the script.
