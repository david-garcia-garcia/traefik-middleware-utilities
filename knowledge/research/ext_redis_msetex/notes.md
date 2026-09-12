# Redis MSETEX

Redis documents `MSETEX` as of **8.4.0**. The argv matches Valkey: `numkeys`, then pairs, then optional `NX`/`XX` and one of `EX`/`PX`/`EXAT`/`PXAT`/`KEEPTTL`. Integer reply `1` if all keys were set, `0` if none were. ([redis.io MSETEX](https://redis.io/docs/latest/commands/msetex/), [.sources/msetex.md](.sources/msetex.md))

This product's compose pin and CI service are **`redis:7-alpine`**, which is Redis 7, not 8.4. Redis 7 has no `MSETEX`. A client that sends the verb receives an **unknown-command** error, not a silent `MSET`-style write.

## Unknown-command wire text (Redis 7 detector)

Redis maps a missing command to a `-` error whose payload starts with `ERR unknown command`. Redis 6.2+ quotes the name and, when args were sent, appends `, with args beginning with:`. ([redis/redis#9107](https://github.com/redis/redis/pull/9107), [.sources/unknown-command.md](.sources/unknown-command.md))

Matching the prefix `ERR unknown command` (after SimpleRedis strips the leading `-`) is enough for capability detection. Do not require the later `with args beginning with` clause — a no-arg unknown command omits it.

## What this means for SimpleRedis

- Compose / CI Redis 7: first `MSETEX` fails unknown-command → Lua `EVAL` fallback. That is the live path to prove (TTL landed).
- Redis 8.4+ (and Valkey 9.1+): the same detector caches native and keeps sending `MSETEX`. Native argv tests stay on the fake; do not add a Redis 8 or Valkey service for this change.

Redis 8.4 also says condition and expiration flags may appear in any order. This client still emits a fixed order: pairs, then `EX`/`EXAT`, then the integer. That order is valid on both Valkey 9.1 and Redis 8.4.
