---
url: https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/distributed-ratelimit/
title: Distributed RateLimit - Traefik (Traefik Proxy docs, v3.7 branch)
fetched: 2026-11-21
authority: official
---

Edit link: traefik/traefik `v3.7` docs/content/reference/routing-configuration/http/middlewares/distributed-ratelimit.md. No "last updated" date on the page.

"This middleware is available exclusively in Traefik Hub." "It is based on a token bucket implementation." Same Rate and Burst text and limit 6 / period 1m / burst 100 example as the Hub reference.

Options that differ from the Hub reference page:
- denyOnError | true — "Whether to deny requests when the rate limit store (Redis) is unavailable. When false, requests are allowed through if Redis cannot be reached." No status code named.
- responseHeaders | false — "(X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset)". No X-RateLimit-Period.
- store.redis.timeout | "" — "Timeout applied to dial, read, and write operations on the Redis connection."
- store.redis.cluster | - — "Set to {} to enable; omit to disable. Cannot be used together with store.redis.sentinel."
- store.redis.sentinel.masterSet — Required "Yes (when using Sentinel)"; cannot be combined with cluster.
- store.redis.tls.ca (not caBundle) — "Custom CA bundle".

Same as Hub reference: limit 0 = no rate limiting, period 1s, burst 1, store.redis.endpoints required, database ignored on Cluster, sourceCriterion fields (no ipv6Subnet), Redis modes single / Cluster / Sentinel.

Not on this page: local cache, sync interval, Redis round trips, Retry-After, header value formats, burst 0.
