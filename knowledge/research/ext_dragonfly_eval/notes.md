# Dragonfly EVAL / Lua differences

Dragonfly exposes Redis-compatible `EVAL` / `EVALSHA` / `SCRIPT` but runs **Lua 5.4.4**, not Redis's Lua 5.1. ([dragonflydb.io Scripting with Lua](https://www.dragonflydb.io/docs/managing-dragonfly/scripting), [.sources/scripting.md](.sources/scripting.md); [dragonfly differences.md](https://github.com/dragonflydb/dragonfly/blob/main/docs/differences.md), [.sources/differences-lua.md](.sources/differences-lua.md))

## Undeclared keys must be declared in KEYS (default)

By default Dragonfly **forbids** scripts from touching keys not listed in the `KEYS`/`numkeys` arguments. Violation returns error `script tried accessing undeclared key` (constant `kUndeclaredKeyErr` in source). ([dragonflydb.io Scripting — Allowing undeclared keys](https://www.dragonflydb.io/docs/managing-dragonfly/scripting#allowing-undeclared-keys), [.sources/scripting.md](.sources/scripting.md); [dragonfly@36eaa12:facade/error.h](https://github.com/dragonflydb/dragonfly/blob/36eaa127c2e5dd4f84724828a4c7c1412c4be90c/src/facade/error.h), [.sources/error-h.md](.sources/error-h.md))

Opt-out: script flag `allow-undeclared-keys` (shebang `--!df flags=…`, server `--default_lua_flags`, or `SCRIPT FLAGS`). Enabling it forces global locking and disables normal multithreaded execution — avoid for hot-path rate limits unless required. ([dragonflydb.io Scripting](https://www.dragonflydb.io/docs/managing-dragonfly/scripting), [.sources/scripting.md](.sources/scripting.md))

This matches Redis's documented rule (all accessed keys must be passed as EVAL key arguments) but Dragonfly **enforces** it by default where Redis historically relied on convention.

## Lua 5.4 vs Redis 5.1 — `table.maxn` absent

| Environment | Lua | `table.maxn` |
|-------------|-----|--------------|
| Redis EVAL | 5.1 | Present ([Lua 5.1 manual — table.maxn](https://www.lua.org/manual/5.1/manual.html#pdf-table.maxn), [.sources/lua51-table-maxn.md](.sources/lua51-table-maxn.md)) |
| Dragonfly EVAL | 5.4.4 | **Not provided** — not in Lua 5.4 stdlib; Dragonfly polyfills `table.getn`/`table.setn`/`foreach`/`foreachi` only, not `maxn` ([dragonfly@36eaa12:interpreter_polyfill.h](https://github.com/dragonflydb/dragonfly/blob/36eaa127c2e5dd4f84724828a4c7c1412c4be90c/src/core/interpreter_polyfill.h), [.sources/interpreter-polyfill.md](.sources/interpreter-polyfill.md)) |

Scripts ported from Redis that call `table.maxn(t)` will fail on Dragonfly unless rewritten (use `#t` for dense sequences, or explicit numeric-key scan for sparse tables).

## Lua 5.4 vs Redis 5.1 — `unpack` / `table.unpack`

| Environment | Lua | Spread a table into args |
|-------------|-----|--------------------------|
| Redis EVAL | 5.1 | Global `unpack` ([Lua 5.1 manual — unpack](https://www.lua.org/manual/5.1/manual.html#pdf-unpack), [.sources/lua51-unpack.md](.sources/lua51-unpack.md)) |
| Dragonfly EVAL | 5.4.4 | `table.unpack` only ([Lua 5.4 manual — table.unpack](https://www.lua.org/manual/5.4/manual.html#pdf-table.unpack), [.sources/lua54-table-unpack.md](.sources/lua54-table-unpack.md)). Dragonfly polyfills `table.getn`/`table.setn`/`foreach`/`foreachi`, **not** `unpack` ([dragonfly@36eaa12:interpreter_polyfill.h](https://github.com/dragonflydb/dragonfly/blob/36eaa127c2e5dd4f84724828a4c7c1412c4be90c/src/core/interpreter_polyfill.h), [.sources/interpreter-polyfill.md](.sources/interpreter-polyfill.md)) |

A script that calls `unpack(...)` runs on Redis and fails on Dragonfly. A script that calls `table.unpack(...)` runs on Dragonfly and fails on Redis 5.1. Shared fallback scripts must use a numeric `for` over `#KEYS` / `#t` and must not call either name.

Dragonfly also documents Lua-integer support not present in Redis 5.1. ([differences.md — Lua](https://github.com/dragonflydb/dragonfly/blob/main/docs/differences.md), [.sources/differences-lua.md](.sources/differences-lua.md))

## Script error wire format

On Lua/`redis.call` failure Dragonfly returns:

```
-ERR Error running script (call to <sha>): <error>
```

([dragonfly@36eaa12:main_service.cc](https://github.com/dragonflydb/dragonfly/blob/36eaa127c2e5dd4f84724828a4c7c1412c4be90c/src/server/main_service.cc), [.sources/main-service-eval-error.md](.sources/main-service-eval-error.md))

**Conflict with latest Redis:** upstream Redis 7.x moved the primary error text ahead of the `Error running script` trailer ([redis/redis#10329](https://github.com/redis/redis/pull/10329)). Dragonfly (v1.40.x) keeps the classic prefix form — closer to pre-7 Redis and antirez release notes.

## EXPIRE flag combination (related compat note)

Dragonfly accepts `EXPIRE`/`EXPIREAT` with `NX` combined with `GT` or `LT`; Redis rejects those combinations as incompatible. Not EVAL-specific but affects shared rate-limit scripts calling `EXPIRE`. ([differences.md — EXPIRE flags](https://github.com/dragonflydb/dragonfly/blob/main/docs/differences.md), [.sources/differences-expire-flags.md](.sources/differences-expire-flags.md))
