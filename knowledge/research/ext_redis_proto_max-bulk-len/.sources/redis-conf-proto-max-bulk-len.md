---
url: https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/redis.conf
title: redis.conf proto-max-bulk-len and client-query-buffer-limit
fetched: 2026-09-12
authority: source
ref: redis/redis@335554f18caf7bbf6b0ac2b3548133d750f00a1b:redis.conf
---

Client query buffers accumulate new commands. Default cap (commented sample):

`client-query-buffer-limit 1gb`

Bulk requests (elements representing single strings) are normally limited to 512 mb. The limit can be changed here but must be 1mb or greater.

`proto-max-bulk-len 512mb` (commented; compiled default still applies)
