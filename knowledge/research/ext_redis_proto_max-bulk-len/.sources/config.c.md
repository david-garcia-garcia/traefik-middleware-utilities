---
url: https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/src/config.c
title: Redis 7.2 config table for proto-max-bulk-len
fetched: 2026-09-12
authority: source
ref: redis/redis@335554f18caf7bbf6b0ac2b3548133d750f00a1b:src/config.c
---

`createLongLongConfig("proto-max-bulk-len", NULL, DEBUG_CONFIG | MODIFIABLE_CONFIG, 1024*1024, LONG_MAX, server.proto_max_bulk_len, 512ll*1024*1024, MEMORY_CONFIG, NULL, NULL)` comment: Bulk request max size.

- min: `1024*1024` (1 MiB)
- max: `LONG_MAX`
- default: `512ll*1024*1024` (536870912)
- runtime `CONFIG SET` allowed (`MODIFIABLE_CONFIG`)
- `MEMORY_CONFIG`: values like `512mb` parse as bytes

Nearby: `client-query-buffer-limit` default `1024*1024*1024` (1 GiB), min 1 MiB.
