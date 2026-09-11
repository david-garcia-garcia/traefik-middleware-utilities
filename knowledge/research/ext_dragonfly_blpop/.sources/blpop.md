---
url: https://www.dragonflydb.io/docs/command-reference/lists/blpop
title: Redis BLPOP Command (Documentation) | Dragonfly
fetched: 2026-09-11
authority: official
---

Syntax: BLPOP key [key ...] timeout

BLPOP is a blocking list pop primitive. It blocks the connection when there are no elements to pop from any of the given lists.

If none of the specified keys exist, BLPOP blocks the connection until another client LPUSH/RPUSH. Non-zero timeout: unblock with a nil multi-bulk when the timeout expires. Timeout is a double in seconds; 0 blocks indefinitely.

Note that the unblock order can differ from Redis when multiple keys receive pushes in one MULTI/EXEC or script. One-key empty-list block is unaffected.

ACL: @write @list @slow @blocking.
