---
url: https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/circuitbreaker/
title: Circuit Breaker
fetched: 2026-09-12
authority: official
---

HTTP circuit breaker: Closed = forward; Open = fallback; does not stack requests on unhealthy services.
Analyzes only what happens after its position in the middleware chain.
Each router gets its own instance; state is not shared across routers.

Defaults (Required: No for all): expression "", checkPeriod 100ms, fallbackDuration 10s, recoveryDuration 10s, responseCode 503.

Metrics: NetworkErrorRatio() e.g. > 0.30; ResponseCodeRatio(from,to,dividedByFrom,dividedByTo) with from inclusive / to exclusive, 0 if denominator 0; LatencyAtQuantileMS(q) with q a float trailing .0, e.g. LatencyAtQuantileMS(50.0) > 100.
Operators: && || and > >= < <= == !=.
Fallback: HTTP 503 unless responseCode is configured.

States: Closed (collect metrics; evaluate expression every checkPeriod); Open (fallback for FallbackDuration, then Recovering); Recovering (linearly increasing requests for RecoveryDuration; fail → Open; survive → Closed).
