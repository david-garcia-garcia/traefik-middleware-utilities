# Redis SELECT out-of-range

Logical databases are zero-based. Dest `redis:7-alpine` default `databases` is **16** (indexes 0–15). `SELECT 15` → `OK`. `SELECT 99` → `ERR DB index is out of range`.

## Command

`SELECT index` switches the connection's logical DB. New connections use database 0. Official return is simple string `OK` on success; docs do not quote the out-of-range text. ([redis.io SELECT](https://redis.io/docs/latest/commands/select/), [.sources/select-command.md](.sources/select-command.md))

## Out-of-range wire error

`selectCommand` calls `selectDb`; on `C_ERR`:

```
DB index is out of range
```

Wire: `-ERR DB index is out of range`. SimpleRedis `replyError` does **not** treat this as AUTH-class; callers see `errors.New("ERR DB index is out of range")`. ([redis@7.2.4:src/db.c `selectCommand`](https://github.com/redis/redis/blob/7.2.4/src/db.c), [.sources/db-select.md](.sources/db-select.md); live `redis:7-alpine` 7.4.10 `CONFIG GET databases` → 16, `SELECT 99`, 2026-09-11)

Passworded handshake: `AUTH secret` then `SELECT 99` still returns that SELECT error (AUTH succeeded first). Same live instance with `--requirepass secret`.
