---
url: https://redis.io/docs/latest/commands/expire/
title: EXPIRE — timeout unchanged by in-place mutations
fetched: 2026-09-11
authority: official
---

The timeout will only be cleared by commands that delete or overwrite the contents of the key, including DEL, SET, GETSET and all the *STORE commands.

All operations that conceptually alter the value stored at the key without replacing it with a new one will leave the timeout untouched. For instance, incrementing the value of a key with INCR, pushing a new value into a list with LPUSH, or altering the field value of a hash with HSET are all operations that will leave the timeout untouched.
