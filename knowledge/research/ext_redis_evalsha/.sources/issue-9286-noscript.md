---
url: https://github.com/redis/redis/issues/9286
title: EVALSHA documented NOSCRIPT vs implementation
fetched: 2026-09-11
authority: vendor
---

Docs once showed backticks around NOSCRIPT. redis-cli on Redis 6.x and 7:

`(error) NOSCRIPT No matching script. Please use EVAL.`

Payload is prefixed with `NOSCRIPT `, not enclosed in backticks. Maintainers treated this as a documentation mismatch, not a wire change.
