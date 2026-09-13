---
url: https://github.com/traefik/traefik/blob/5b53bae42d2dab453b8f932db760b874f99ee984/pkg/config/dynamic/middlewares.go
title: pkg/config/dynamic/middlewares.go CircuitBreaker
fetched: 2026-09-12
authority: source
ref: traefik/traefik@5b53bae42d2dab453b8f932db760b874f99ee984:pkg/config/dynamic/middlewares.go
---

type CircuitBreaker: Expression string; CheckPeriod, FallbackDuration, RecoveryDuration ptypes.Duration; ResponseCode int.
SetDefaults: CheckPeriod = 100ms; FallbackDuration = 10s; RecoveryDuration = 10s; ResponseCode = 503 (StatusServiceUnavailable).
Expression is not set in SetDefaults (stays "").
