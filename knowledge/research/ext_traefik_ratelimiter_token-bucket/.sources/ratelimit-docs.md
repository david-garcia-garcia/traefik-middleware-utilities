---
url: https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/ratelimit/
title: RateLimit middleware
fetched: 2026-09-11
authority: official
---

Token bucket. average/period = refill rate. burst = bucket size. average 0 = no limit.
period default 1s, burst default 1. Redis optional for shared tokens across instances.
No delay/maxDelay config field. sourceCriterion is HTTP source grouping.
