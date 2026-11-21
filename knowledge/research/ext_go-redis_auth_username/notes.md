# go-redis Username without Password

What go-redis v9 does with `Options.Username` on connection init.

- Both the `HELLO ... AUTH` path and the legacy `AUTH` fallback run only when `Password != ""`. With a password, a non-empty `Username` selects the two-argument form (`AUTH <username> <password>`); an empty one sends `AUTH <password>` (HELLO uses `default`).
- A `Username` with an empty `Password` sends no authentication at all: the username is silently ignored, and the connection runs as the server's `default` user.

Source: `github.com/redis/go-redis@v9.22.0:redis.go` (`initConn`), `commands.go` (`Hello`, `AuthACL`), `options.go` (`Username`); `.sources/redis.go.md` (read from the local module cache, not cloned). Redis behaviour for `AUTH <user> ""` is in `ext_redis_auth`.
