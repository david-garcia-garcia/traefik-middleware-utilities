# Dragonfly EVALSHA and NOSCRIPT

Dragonfly exposes Redis-compatible `EVALSHA` / `SCRIPT` and runs **Lua 5.4.4**. Digest lookup and the miss error are Redis-shaped. Undeclared keys and `table.maxn` stay on `ext_dragonfly_eval`. ([dragonflydb.io EVALSHA](https://www.dragonflydb.io/docs/command-reference/generic/evalsha), [.sources/evalsha-command.md](.sources/evalsha-command.md); [dragonfly@v1.40.2:src/facade/error.h](https://github.com/dragonflydb/dragonfly/blob/v1.40.2/src/facade/error.h), [.sources/error-h-noscript.md](.sources/error-h-noscript.md))

## Command shape

```
EVALSHA sha1 numkeys [key [key ...]] [arg [arg ...]]
```

Official command page: evaluate a script from the server cache by SHA1 digest; cache via `SCRIPT LOAD`; otherwise identical to `EVAL`. ([EVALSHA](https://www.dragonflydb.io/docs/command-reference/generic/evalsha), [.sources/evalsha-command.md](.sources/evalsha-command.md))

Compatibility matrix: `EVAL`, `EVALSHA`, `SCRIPT LOAD` fully supported. ([dragonflydb.io compatibility](https://www.dragonflydb.io/docs/command-reference/compatibility) via `ext_dragonfly_container-image`)

## NOSCRIPT wire text (v1.40.2)

```
-NOSCRIPT No matching script. Please use EVAL.
```

Constant `facade::kScriptNotFound`. After `readReply` strips the leading `-`, SimpleRedis `replyError` text is `NOSCRIPT No matching script. Please use EVAL.` — same prefix Redis 7 sends. ([dragonfly@v1.40.2:src/facade/error.h](https://github.com/dragonflydb/dragonfly/blob/v1.40.2/src/facade/error.h), [.sources/error-h-noscript.md](.sources/error-h-noscript.md))

A client `strings.HasPrefix(err, "NOSCRIPT")` matches Redis 7 and this pin. Empty or invalid SHA also takes this path (later empty-SHA crash fix still returns NOSCRIPT rather than a different code).

## SCRIPT FLUSH / EXISTS

`SCRIPT EXISTS` returns 1/0 per digest. `SCRIPT FLUSH` drops the cache so a following `EVALSHA` is a miss. Same RESP as Redis; Pester can drive both from `redis:7-alpine`'s `redis-cli` using `-h redis` and `-h dragonfly` on the compose network. ([eval-intro SCRIPT command](https://redis.io/docs/latest/develop/programmability/eval-intro/) via `ext_redis_evalsha`; Dragonfly command pages [SCRIPT EXISTS](https://www.dragonflydb.io/docs/command-reference/generic/script-exists), [.sources/script-exists.md](.sources/script-exists.md))

## SHA-1 agreement

Client-side SHA-1 of the script bytes (Redis `sha1hex`, go-redis `crypto/sha1` + hex) must equal the engine's digest or `SCRIPT EXISTS` of that hex is `0` after `EVAL`. Early Dragonfly (issue 59) hashed some scripts differently than Redis; **v1.40.2** is treated as Redis-compatible. Live `SCRIPT EXISTS` of the Go digest is the check — do not special-case a second hash for Dragonfly. ([ext_redis_evalsha](../ext_redis_evalsha/notes.md); [dragonfly#59](https://github.com/dragonflydb/dragonfly/issues/59), [.sources/issue-59-sha.md](.sources/issue-59-sha.md))
