---
url: https://doc.traefik.io/traefik-hub/api-gateway/expose/middleware/distributed-rate-limit
title: Distributed RateLimit | Traefik Hub Documentation (guide)
fetched: 2026-11-21
authority: official
---

Page footer: "Last updated on Nov 18, 2026".

"It is based on the Token Bucket algorithm." Limits apply "throughout your cluster and not only on an individual proxy."

"In Traefik Hub API Gateway, the Distributed RateLimit middleware requires you install a Redis() instance in your cluster." Helm install example, password in a Kubernetes Secret.
Supported Redis modes: single instance, Redis Cluster, Redis Sentinel. `database` ignored on Cluster.

Example (burst 200, limit 100, period 1s, denyOnError false, responseHeaders true, sourceCriterion.ipStrategy {}, store.redis.timeout 500ms) with comments:
"each IP address will get its own bucket."
"Buckets are refilled at a rate of 100 tokens per second and a maximum capacity of 200 tokens."
"It allows someone to query on average at a rate of 100 requests per second and at most 200 simultaneous request temporary."

"Rate Limit not distributed": with one gateway instance, "or if you don't want the share the rate limit bucket between your instances, You can use the RateLimit middleware."

Advanced options deferred to the reference page. No text on denyOnError, 429, headers, Retry-After, local cache, sync, round trips, burst 0.
