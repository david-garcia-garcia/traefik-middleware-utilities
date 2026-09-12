---
url: https://redis.io/docs/latest/commands/exec/
title: EXEC
fetched: 2026-09-11
authority: official
---

EXEC runs queued transaction commands.

RESP2 return:
- Array reply: one element per queued command.
- Nil reply: the transaction was aborted because a WATCHed key was touched.

RESP3 return:
- Array reply, or Null reply for the same abort case.
