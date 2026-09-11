---
url: https://redis.io/docs/latest/develop/using-commands/pipelining/
title: Redis pipelining
fetched: 2026-09-11
authority: official
---

Pipelining: issue multiple commands at once without waiting for each response. Redis has supported this since early days.

Sequential four INCR: client INCR, server 1, client INCR, server 2, … Pipelined: client sends four INCR, then server returns 1 2 3 4.

netcat example: `(printf "PING\r\nPING\r\nPING\r\n"; sleep 1) | nc localhost 6379` → three `+PONG`.

IMPORTANT NOTE: server queues replies in memory. For a lot of commands, send batches of a reasonable size (example 10k), read replies, then send the next 10k.

EVAL/EVALSHA in a pipeline is entirely possible. SCRIPT LOAD guarantees EVALSHA can be called without failing (separate from pipelining itself). Pipelining cannot help when the client must read before the next write; use scripting for read-compute-write.
