---
url: https://github.com/golang/time/blob/v0.15.0/rate/rate.go
title: golang.org/x/time/rate token bucket
fetched: 2026-09-11
authority: source
ref: golang.org/x/time@v0.15.0:rate/rate.go
---

Traefik in-memory store is this limiter (go.mod `golang.org/x/time v0.15.0`). Token bucket of size b, refilled at r tokens/s; Inf ignores burst.

`NewLimiter`: tokens start at burst (full).
`Reserve()` = `ReserveN(now, 1)`. Advance: if t < last, last=t; elapsed * rate; min burst; tokens -= n; if tokens < 0 wait = durationFromTokens(-tokens).
`Cancel()` restores tokens from that reservation. Traefik calls this when Delay() > maxDelay.
Same clock as Traefik Lua (units differ: seconds vs microseconds).
