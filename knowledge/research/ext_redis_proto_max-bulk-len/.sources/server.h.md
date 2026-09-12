---
url: https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/src/server.h
title: Redis protocol size macros and proto_max_bulk_len field
fetched: 2026-09-12
authority: source
ref: redis/redis@335554f18caf7bbf6b0ac2b3548133d750f00a1b:src/server.h
---

`PROTO_INLINE_MAX_SIZE` is `(1024*64)` — max size of inline reads (and of unread `*`/`$` header lines before `\r`).

`PROTO_MBULK_BIG_ARG` is `(1024*32)` — threshold to trim/hint querybuf for a large bulk, not a rejection cap.

`server.proto_max_bulk_len` comment: Protocol bulk length maximum size.
