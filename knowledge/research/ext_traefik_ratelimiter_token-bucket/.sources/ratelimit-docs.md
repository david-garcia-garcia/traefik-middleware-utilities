---
url: https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/ratelimit/
title: RateLimit middleware
fetched: 2026-11-21
authority: official
---

Docs branch on 2026-11-21: edit link points at traefik/traefik v3.7 docs/content/reference/routing-configuration/http/middlewares/ratelimit.md. No "last updated" date on page. Core facts unchanged since the 2026-09-11 read.

Token bucket. average/period = refill rate. burst = bucket size. average 0 = no limit.
period default 1s, burst default 1. Redis optional for shared tokens across instances.
No delay/maxDelay config field. sourceCriterion is HTTP source grouping (requestHost, requestHeaderName, ipStrategy.depth / excludedIPs / ipv6Subnet).

redis row: "enables distributed rate limiting by using Redis to store rate limit tokens across multiple Traefik instances." "When Redis is not configured, Traefik uses in-memory storage for rate limiting, which works only for the individual Traefik instance."
redis options (default): endpoints ("localhost:6379"; "multiple endpoints for Redis cluster or high availability setups"), username (""), password (""), db (0), poolSize (0 → 10 per GOMAXPROCS core), minIdleConns (0), maxActiveConns (0 = no limit), readTimeout (3s), writeTimeout (3s), dialTimeout (5s), tls.ca (""; system bundle), tls.cert (""), tls.key (""), tls.insecureSkipVerify (false). Kubernetes example also uses secret / tls.caSecret / tls.certSecret (no table rows).
No sentinel or cluster field. No denyOnError, no text on behaviour when Redis is unreachable, no Retry-After, no X-RateLimit-* headers.
