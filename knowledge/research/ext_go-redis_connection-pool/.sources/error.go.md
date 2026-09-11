---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/error.go
title: redis.ErrPoolTimeout re-export
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:error.go
---

`var ErrPoolTimeout = pool.ErrPoolTimeout` with comment "timed out waiting to get a connection from the connection pool."

shouldRetry: `errors.Is(err, pool.ErrPoolTimeout)` is true — pool timeout is retried (#3289). Context cancel/deadline is not.
