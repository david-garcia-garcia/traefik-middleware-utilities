---
url: https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/src/networking.c
title: processMultibulkBuffer bulk-length and multibulk checks
fetched: 2026-09-12
authority: source
ref: redis/redis@335554f18caf7bbf6b0ac2b3548133d750f00a1b:src/networking.c
---

`processMultibulkBuffer` parses a RESP array-of-bulks command (`*` then `$` args).

`*` count (`multibulklen`):
- `string2ll`; `!ok || ll > INT_MAX` → `Protocol error: invalid multibulk length`
- `ll > 10 && authRequired(c)` → `unauthenticated multibulk length`
- `ll <= 0` is OK (empty / null-style); not `proto-max-bulk-len`

Each `$` bulk length:
- `string2ll` into `ll`
- reject when `!ok || ll < 0 || (!(c->flags & CLIENT_MASTER) && ll > server.proto_max_bulk_len)`
- error: `Protocol error: invalid bulk length`; connection marked protocol-error and close-after-reply
- `CLIENT_MASTER` skips `proto_max_bulk_len`
- `ll > 16384 && authRequired(c)` → `unauthenticated bulk length`

Header line without `\r` longer than `PROTO_INLINE_MAX_SIZE`: `too big mbulk count string` / `too big bulk count string`.
