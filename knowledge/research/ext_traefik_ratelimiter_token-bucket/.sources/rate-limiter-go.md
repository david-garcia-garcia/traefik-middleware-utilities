---
url: https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/middlewares/ratelimiter/rate_limiter.go
title: pkg/middlewares/ratelimiter/rate_limiter.go
fetched: 2026-09-11
authority: source
ref: traefik/traefik@903e8a965795db5e750004ff74932983e269b85f:pkg/middlewares/ratelimiter/rate_limiter.go
---

rtl = average * Second / period. maxDelay = 1/(2*rtl) or 500ms if rtl<1. ttl ~1s plus slack.
limiter.Allow: err→500, nil delay→429, delay>maxDelay→429 Retry-After (no sleep), else time.After(delay).
Source extractor + name:source key are HTTP. Redis vs in-memory chosen by config.Redis != nil.
