# Redis AUTH error strings

Handshake `AUTH` replies this product maps (or does not map) through `replyError`. Dest image is `redis:7-alpine` (`redis_version` **7.4.10** measured 2026-09-11).

## One-argument AUTH is the default user

SimpleRedis sends `AUTH <password>` only. Redis treats the omitted username as `default`. Official AUTH docs: single-argument form authenticates against `requirepass` / ACL user `default`. ([redis.io AUTH](https://redis.io/docs/latest/commands/auth/), [.sources/auth-command.md](.sources/auth-command.md); [redis.io security — Authentication](https://redis.io/docs/latest/operate/oss_and_stack/management/security/), [.sources/security.md](.sources/security.md))

## Wrong password → `WRONGPASS` (maps to `redis:noauth`)

With `requirepass` set, a bad password returns:

```
WRONGPASS invalid username-password pair or user is disabled.
```

Wire: `-WRONGPASS …`. SimpleRedis `replyError` prefixes include `WRONGPASS` → `redis:noauth`. ([redis@7.2.4:src/acl.c `addAuthErrReply`](https://github.com/redis/redis/blob/7.2.4/src/acl.c), [.sources/acl-auth.md](.sources/acl-auth.md); live `redis:7-alpine` 7.4.10 `AUTH wrong` against `--requirepass secret`, 2026-09-11)

Unauthenticated `PING` on that same instance returns `NOAUTH Authentication required.` (command path, not AUTH itself). AUTH is `no_auth`; the AUTH command does not reply `NOAUTH`.

## AUTH when default user is nopass → not `redis:noauth` on Redis 7.4

`authCommand` two-argument form, if `DefaultUser` has `USER_FLAG_NOPASS`:

```
AUTH called without any password configured for the default user. Are you sure your configuration is correct?
```

Wire: `-ERR AUTH <password> called without any password configured for the default user. Are you sure your configuration is correct?`

SimpleRedis prefix `ERR Client sent AUTH` does **not** match this 7.x text. Live 7.4.10 matched the 7.2.4 source string. Historical wording `ERR Client sent AUTH, but no password is set` is not what `redis:7-alpine` emits. ([redis@7.2.4:src/acl.c `authCommand`](https://github.com/redis/redis/blob/7.2.4/src/acl.c), [.sources/acl-auth.md](.sources/acl-auth.md); live `AUTH wrong` against no-`requirepass` `redis:7-alpine`, 2026-09-11)

## NOPERM is not an AUTH handshake reply

`NOPERM` is an ACL denial of a later command. AUTH itself is allowed without prior auth. Fake handshake tests may still send a `NOPERM` AUTH reply to cover the prefix; live Redis does not emit it from `AUTH`.
