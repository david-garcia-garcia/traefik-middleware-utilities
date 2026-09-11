---
url: https://redis.io/docs/latest/commands/blpop/
title: BLPOP
fetched: 2026-09-11
authority: official
---

BLPOP is a blocking list pop primitive. It blocks the connection when there are no elements to pop from any of the given lists.

Syntax: BLPOP key [key ...] timeout

timeout is the maximum time to block, in seconds. A timeout of 0 blocks indefinitely. Interpreted as a double.

If none of the specified keys exist, BLPOP blocks the connection until another client LPUSH/RPUSH, or until timeout. When a non-zero timeout expires without a push, the client unblocks with a nil multi-bulk.

Return (RESP2): Nil reply when timeout expired; Array reply of key name and popped element.

command_flags include write and blocking. ACL: @write @list @slow @blocking. Since 2.0.0.
