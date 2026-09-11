---
url: https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/middlewares/ratelimiter/redis_limiter.go
title: pkg/middlewares/ratelimiter/redis_limiter.go
fetched: 2026-09-11
authority: source
ref: traefik/traefik@903e8a965795db5e750004ff74932983e269b85f:pkg/middlewares/ratelimiter/redis_limiter.go
---

redisPrefix = "rate:". go-redis UniversalClient. script.Run (EVALSHA/EVAL).
ARGV: rate/1e6, burst, ttl, UnixMicro, maxDelay.Microseconds().
Allow: script error returned; ok false → nil delay; else &microDelay.
rate.Inf short-circuits allow with no Redis call. No denyOnError in this file.
