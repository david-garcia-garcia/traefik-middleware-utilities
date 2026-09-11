---
url: https://redis.io/docs/latest/develop/clients/hiredis/transpipe/
title: Pipelines and transactions
fetched: 2026-09-11
authority: official
---

Two batch types: pipelines (several commands in one communication, all responses back) vs transactions (MULTI/EXEC, no interruption by other clients).

hiredis has no start-pipeline command. `redisAppendCommand` adds to an output buffer without sending. First `redisGetReply` sends queued commands then returns the first reply. Later `redisGetReply` calls drain remaining replies.

Example pipelines six commands (three SET, three GET). Loop `for i in 0..5` calls `redisGetReply` once per command. Per reply: if `reply->type == REDIS_REPLY_ERROR` print the error, else print the value. Free each reply.

Call `redisGetReply` once for each command added. Check errors after each call.
