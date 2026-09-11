---
url: https://redis.io/docs/latest/commands/incr/
title: INCR
fetched: 2026-09-11
authority: official
---

Increments the number stored at key by one.
If the key does not exist, it is set to 0 before performing the operation.
An error is returned if the key contains a value of the wrong type or contains a string that can not be represented as integer.
Limited to 64 bit signed integers.

Return information (RESP2/RESP3): Integer reply — the value of the key after the increment.

Example: SET mykey "10" → INCR mykey → (integer) 11.
