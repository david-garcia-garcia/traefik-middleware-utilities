# Dragonfly has no MSETEX

Dragonfly's published command-compatibility matrix (verified against **Dragonfly v1.40.0**, the line this repo pins as `v1.40.2`) lists String commands `APPEND`, `GET`, `GETEX`, `MGET`, `MSET`, `MSETNX`, `SET` (partial: missing Redis 8.4 `IFEQ`/`IFNE`/`IFDEQ`/`IFDNE`), `SETEX`, `SETNX`, and related increment/range verbs. **`MSETEX` is not in the table.** ([dragonflydb.io API Compatibility](https://www.dragonflydb.io/docs/command-reference/compatibility), [.sources/compatibility-string.md](.sources/compatibility-string.md))

Absence from that matrix means Dragonfly v1.40.x does not accept `MSETEX`. A client that sends it receives an unknown-command error, same detector as Redis 7.

`SET` with `EX` / `EXAT` is listed as supported (the missing SET flags are the Redis 8.4 IF* predicates, not expiration). The Lua fallback `redis.call('SET', KEYS[i], ARGV[i], 'EX'|'EXAT', ttl)` is therefore on a supported command, provided every key is declared in `EVAL` `KEYS` (`ext_dragonfly_eval`).
