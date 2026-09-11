---
url: https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/config/dynamic/middlewares.go
title: pkg/config/dynamic/middlewares.go RateLimit
fetched: 2026-09-11
authority: source
ref: traefik/traefik@903e8a965795db5e750004ff74932983e269b85f:pkg/config/dynamic/middlewares.go
---

`RateLimit` fields: Average, Period, Burst, SourceCriterion, Redis. No Delay.
Average 0 = no limiting (docs / in-memory); rate = Average / Period; Period default 1s; Burst default 1; Redis omitted → in-memory bucket.
