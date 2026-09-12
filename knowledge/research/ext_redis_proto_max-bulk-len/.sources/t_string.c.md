---
url: https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/src/t_string.c
title: checkStringLength SETRANGE and APPEND
fetched: 2026-09-12
authority: source
ref: redis/redis@335554f18caf7bbf6b0ac2b3548133d750f00a1b:src/t_string.c
---

`checkStringLength(client *c, long long size, long long append)`:
- `mustObeyClient(c)` → `C_OK` (replicas apply the write)
- `total = (uint64_t)size + append`
- reject when `total > server.proto_max_bulk_len` or overflow (`total < size || total < append`)
- error: `string exceeds maximum allowed size (proto-max-bulk-len)`

Callers: `setrangeCommand` (resulting length `offset + sdslen(value)`), `appendCommand` (existing length + append payload).
