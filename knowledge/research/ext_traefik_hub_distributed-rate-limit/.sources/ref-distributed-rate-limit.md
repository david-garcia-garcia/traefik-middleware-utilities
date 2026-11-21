---
url: https://doc.traefik.io/traefik-hub/api-gateway/reference/routing/http/middlewares/ref-distributed-rate-limit
title: Distributed RateLimit | Traefik Hub Documentation (reference)
fetched: 2026-11-21
authority: official
---

Page footer: "Last updated on Nov 18, 2026".

Intro: "ensures that requests are limited over time throughout your cluster and not only on an individual proxy." "It is based on a token bucket implementation."

Config is `spec.plugin.distributedRateLimit` on a `traefik.io/v1alpha1` Middleware.

Rate and Burst: "The rate is defined by dividing limit by period." "For a rate below 1 req/s, define a period larger than a second". "the limit and period parameters define the rate at which the bucket refills, and the burst is the size (volume) of the bucket."
Example burst 100 / period 1m / limit 6: "allows up to 100 connections in parallel (burst)"; "once the 100 tokens are consumed, the other ones are blocked until at least one token is available"; "on token is generated every 10 seconds" (typo on page).

Options table (field | default | required):
- limit | 0 | No — "0 means no rate limiting."
- period | 1s | No
- burst | 1 | No — "Maximum number of requests allowed to go through at the very same moment."
- denyOnError | true | No — "Forces to return a 429 error if the number of remaining requests accepted cannot be get. Set to false, this option allows the request to reach the backend."
- responseHeaders | false | No — injects X-RateLimit-Remaining, X-RateLimit-Limit, X-RateLimit-Period, X-RateLimit-Reset. "The added headers indicate how many tokens are left in the bucket (in the token bucket analogy) after the reservation for the request was made."
- store.redis.endpoints | "" | Yes
- store.redis.username, store.redis.password | "" | No
- store.redis.database | 0 | No
- store.redis.cluster | "" | No — "Enable Redis Cluster"
- store.redis.tls.caBundle, tls.cert, tls.key | "" | No; tls.insecureSkipVerify | false | No
- store.redis.sentinel.masterSet | "" | No — description says "Required when using Sentinel."
- store.redis.sentinel.username, sentinel.password | "" | No
- sourceCriterion.requestHost | false; requestHeaderName | ""; ipStrategy.depth | 0; ipStrategy.excludedIPs | (blank). No ipv6Subnet row.

Example config also sets `store.redis.timeout: 500ms`; `timeout` has no row in the options table.

sourceCriterion: several strategies at once → "an error will be raised"; none set → "the request's remote address field (as an ipStrategy)".

store: "uses a persistent KV storage to store data." Supported Redis modes: single instance, Redis Cluster, Redis Sentinel. `database` ignored on Cluster (only 0), "a warning is displayed".

Not on this page: local cache, sync interval between instances, Redis round trips per request, Retry-After, header value formats/units, burst 0 behaviour, status code for a request denied by the limit (429 appears only under denyOnError), any delay/wait-before-deny behaviour, Redis key names or TTL.
