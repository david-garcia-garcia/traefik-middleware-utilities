---
url: https://github.com/traefik/traefik/pull/10211
title: Add Redis rate limiter
fetched: 2026-09-11
authority: comment
---

Review: do not ship GCRA (go-redis/redis-rate) next to in-memory x/time/rate token bucket.
Same RateLimit config struct must mean the same algorithm whether Redis is set or not.
PR migrated Redis to the token-bucket Lua that landed.
