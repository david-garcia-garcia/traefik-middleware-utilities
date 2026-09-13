---
url: https://github.com/traefik/traefik/blob/5b53bae42d2dab453b8f932db760b874f99ee984/pkg/middlewares/circuitbreaker/circuit_breaker.go
title: pkg/middlewares/circuitbreaker/circuit_breaker.go
fetched: 2026-09-12
authority: source
ref: traefik/traefik@5b53bae42d2dab453b8f932db760b874f99ee984:pkg/middlewares/circuitbreaker/circuit_breaker.go
---

Delegates to github.com/vulcand/oxy/v2/cbreaker (Traefik v3.3.0 go.mod: v2.0.0).
Always sets Fallback: WriteHeader(responseCode) + StatusText body; logs blocked-by-circuit-breaker.
Passes CheckPeriod, FallbackDuration, RecoveryDuration to oxy only when the config duration is > 0.
cbreaker.New(next, expression, opts...); New error is returned as-is.
ServeHTTP is a straight pass-through to the oxy breaker.
