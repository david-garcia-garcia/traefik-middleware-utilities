---
url: https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/middlewares/ratelimiter/in_memory_limiter.go
title: pkg/middlewares/ratelimiter/in_memory_limiter.go
fetched: 2026-09-11
authority: source
ref: traefik/traefik@903e8a965795db5e750004ff74932983e269b85f:pkg/middlewares/ratelimiter/in_memory_limiter.go
---

x/time/rate Limiter per source in ttlmap. Reserve(); not OK → nil delay.
Delay()>maxDelay → Cancel() refund; still returns &delay. HTTP 429s on that delay.
