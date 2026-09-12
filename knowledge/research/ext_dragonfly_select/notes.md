# Dragonfly SELECT out-of-range

Dest `dragonfly:v1.40.2` `--dbnum` default is **16** (same shape as Redis `databases`). `SELECT 15` → `OK`. `SELECT 99` → `ERR DB index is out of range`.

## Command

Official SELECT: zero-based index, new connections use DB 0. Return section does not quote errors. ([dragonfly SELECT](https://www.dragonflydb.io/docs/command-reference/server-management/select), [.sources/select-command.md](.sources/select-command.md))

`--dbnum` "Number of databases", default 16. ([dragonfly flags](https://www.dragonflydb.io/docs/managing-dragonfly/flags), [.sources/flags.md](.sources/flags.md); `CONFIG GET dbnum` example shows `"16"` in [CONFIG GET docs](https://www.dragonflydb.io/docs/command-reference/server-management/config-get), [.sources/config-get.md](.sources/config-get.md))

## Live v1.40.2 (2026-09-11)

No-password and `--requirepass=secret` instances both:

- `CONFIG GET dbnum` → `dbnum` / `16`
- `SELECT 15` → `OK`
- `SELECT 99` → `ERR DB index is out of range`

Passworded: `AUTH secret` then `SELECT 99` still yields that SELECT error. Same wire as Redis 7.4.10. SimpleRedis surfaces it as `ERR DB index is out of range`, not `redis:noauth`.

A live bad-DB case does **not** need a extra Dragonfly; dest's unpassworded `dragonfly:6379` already rejects `SELECT 99`.
