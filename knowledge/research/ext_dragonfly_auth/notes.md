# Dragonfly AUTH vs requirepass

Dest image `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` (`dragonfly_version` **df-v1.40.2** measured 2026-09-11). Official AUTH is Redis-compatible; `--requirepass` sets the ACL `default` user password. ([dragonfly AUTH](https://www.dragonflydb.io/docs/command-reference/server-management/auth), [.sources/auth-command.md](.sources/auth-command.md); [dragonfly ACL](https://www.dragonflydb.io/docs/managing-dragonfly/acl), [.sources/acl.md](.sources/acl.md); [dragonfly flags `--requirepass`](https://www.dragonflydb.io/docs/managing-dragonfly/flags), [.sources/flags.md](.sources/flags.md))

## Default (no requirepass): AUTH any password succeeds

Official ACL: user `default` can AUTH using **any** password unless that user is given a password or turned `OFF`. Live v1.40.2 with no `--requirepass`: `AUTH wrong` → `OK`, `PING` → `PONG`. ([dragonfly ACL — Authentication](https://www.dragonflydb.io/docs/managing-dragonfly/acl), [.sources/acl.md](.sources/acl.md); live 2026-09-11)

This **differs from Redis 7.4**: Redis nopass rejects one-argument AUTH with `ERR AUTH <password> called without any password configured…`. Dragonfly nopass AUTH is not a live wrong-password case.

## With `--requirepass`: same WRONGPASS / NOAUTH as Redis 7.4

`--requirepass` default `""`; when set, it is the `default` user password. ([flags `--requirepass`](https://www.dragonflydb.io/docs/managing-dragonfly/flags), [.sources/flags.md](.sources/flags.md))

Live v1.40.2 `--requirepass=secret`:

| Command | Reply |
|---------|--------|
| `PING` (no AUTH) | `NOAUTH Authentication required.` |
| `AUTH wrong` | `WRONGPASS invalid username-password pair or user is disabled.` |
| `AUTH secret` | `OK` |

WRONGPASS maps to SimpleRedis `redis:noauth`. A live wrong-password case **requires** a passworded Dragonfly; the dest compose service today has no `--requirepass`.
