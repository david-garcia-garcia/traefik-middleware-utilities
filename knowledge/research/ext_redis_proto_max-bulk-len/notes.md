# proto-max-bulk-len

Default and meaning of Redis `proto-max-bulk-len`. Pinned clone: `https://github.com/redis/redis` @ `335554f18caf7bbf6b0ac2b3548133d750f00a1b` (shallow `7.2`, tag `7.2.16`). Dest compose `redis:7-alpine` is this 7.x line.

## Default is 512 MiB

The compiled default is `512ll*1024*1024` (**536870912** bytes, 512 MiB). `CONFIG SET` / `redis.conf` accept memory units (`512mb`). The floor is **1 MiB** (`1024*1024`); values below that are rejected. ([redis.io protocol spec — Bulk strings](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings), [.sources/protocol-spec-bulk-strings.md](.sources/protocol-spec-bulk-strings.md); [redis.conf `proto-max-bulk-len 512mb`](https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/redis.conf), [.sources/redis-conf-proto-max-bulk-len.md](.sources/redis-conf-proto-max-bulk-len.md); [redis/redis@335554f:src/config.c](https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/src/config.c) `createLongLongConfig("proto-max-bulk-len"…)`, [.sources/config.c.md](.sources/config.c.md))

The sample `redis.conf` line is commented; the default still applies. Directives live in that file, not a separate redis.io table. ([redis.io Redis configuration](https://redis.io/docs/latest/operate/oss_and_stack/management/config/), [.sources/redis-configuration.md](.sources/redis-configuration.md))

## What it limits: one incoming bulk string, not array arity

`proto-max-bulk-len` is the max **declared length of one `$` bulk string** in an incoming client command (`$<len>\r\n<data>\r\n`). `processMultibulkBuffer` rejects `ll < 0` or `ll > server.proto_max_bulk_len` with `Protocol error: invalid bulk length` and closes the client. Replica/master clients (`CLIENT_MASTER`) skip the cap so a replication stream can carry larger bulks. ([redis/redis@335554f:src/networking.c](https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/src/networking.c) `processMultibulkBuffer`, [.sources/networking.c.md](.sources/networking.c.md))

It does **not** cap `*` array arity. The `*` count is `string2ll` then `ll > INT_MAX` → `invalid multibulk length`. Unauthenticated clients also cannot send `*` count `> 10`. Inline (non-`*`) lines are a different cap: `PROTO_INLINE_MAX_SIZE` = 64 KiB. ([redis/redis@335554f:src/networking.c](https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/src/networking.c), [.sources/networking.c.md](.sources/networking.c.md); [redis/redis@335554f:src/server.h](https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/src/server.h) `PROTO_INLINE_MAX_SIZE`, [.sources/server.h.md](.sources/server.h.md))

A whole command can still be large when many arguments stay under the bulk cap; `client-query-buffer-limit` (default 1 GiB) is the query-buffer ceiling, not this finding. ([redis.conf `client-query-buffer-limit`](https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/redis.conf), [.sources/redis-conf-proto-max-bulk-len.md](.sources/redis-conf-proto-max-bulk-len.md))

## SETRANGE and APPEND reuse the same number

After a command is parsed, `checkStringLength` refuses a resulting string larger than `server.proto_max_bulk_len` (overflow-safe). Callers: `setrangeCommand`, `appendCommand`. Error text: `string exceeds maximum allowed size (proto-max-bulk-len)`. Masters skip this check (`mustObeyClient`). SET of a huge value is already blocked at the `$` parse. ([redis/redis@335554f:src/t_string.c](https://github.com/redis/redis/blob/335554f18caf7bbf6b0ac2b3548133d750f00a1b/src/t_string.c) `checkStringLength`, [.sources/t_string.c.md](.sources/t_string.c.md))

## Client replies

The server emits GET (and similar) bulk replies from stored objects. It does not re-apply `proto-max-bulk-len` on the way out. A client that trusts `$<len>` on the reply path is not protected by this server config.
