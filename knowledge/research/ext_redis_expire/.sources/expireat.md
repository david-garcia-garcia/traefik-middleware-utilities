---
url: https://redis.io/docs/latest/commands/expireat/
title: EXPIREAT
fetched: 2026-09-11
authority: official
---

EXPIREAT has the same effect and semantic as EXPIRE, but instead of specifying the number of seconds representing the TTL, it takes an absolute Unix timestamp (seconds since January 1, 1970). A timestamp in the past will delete the key immediately.

Required arguments: key, unix-time-seconds.

Return information (RESP2/RESP3), one of:
- Integer reply: 0 if the timeout was not set; for example, the key doesn't exist, or the operation was skipped because of the provided arguments.
- Integer reply: 1 if the timeout was set.

Example: SET mykey "Hello" → EXPIREAT mykey 1293840000 → (integer) 1 → EXISTS mykey → (integer) 0.
