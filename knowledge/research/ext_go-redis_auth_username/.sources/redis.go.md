---
url: https://github.com/redis/go-redis/blob/v9.22.0/redis.go
title: go-redis v9.22.0 initConn HELLO/AUTH and Options.Username
fetched: 2026-11-21
authority: source
ref: github.com/redis/go-redis@v9.22.0:redis.go, commands.go, options.go
---

options.go: `// Username is used to authenticate the current connection with one of the connections defined in the ACL list when connecting to a Redis 6.0 instance, or greater, that is using the Redis ACL system.` `Username string`

commands.go `Hello`: `if password != "" { if username != "" { args = append(args, "auth", username, password) } else { args = append(args, "auth", "default", password) } }`

redis.go `initConn` fallback after HELLO is rejected: `if password != "" { if username != "" { initErr = conn.AuthACL(ctx, username, password).Err() } else { initErr = conn.Auth(ctx, password).Err() } }`. `AuthACL` sends `auth <username> <password>`.
