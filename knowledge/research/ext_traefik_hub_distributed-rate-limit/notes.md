# Traefik Hub DistributedRateLimit

Facts about Traefik Hub's `distributedRateLimit` middleware as **documented** (Hub is a commercial product; no source was cloned or inspected, so internals are unknown). Not Traefik Proxy OSS `rateLimit` (`ext_traefik_ratelimiter_token-bucket/`). Not this product's wrappers. Every fact below is docs-only, read 2026-11-21.

Owners:

- Hub reference — [ref-distributed-rate-limit](https://doc.traefik.io/traefik-hub/api-gateway/reference/routing/http/middlewares/ref-distributed-rate-limit), "Last updated on Nov 18, 2026" ([.sources/ref-distributed-rate-limit.md](.sources/ref-distributed-rate-limit.md)).
- Hub guide — [expose/middleware/distributed-rate-limit](https://doc.traefik.io/traefik-hub/api-gateway/expose/middleware/distributed-rate-limit), "Last updated on Nov 18, 2026" ([.sources/distributed-rate-limit-guide.md](.sources/distributed-rate-limit-guide.md)).
- Proxy docs copy — [traefik/.../distributed-ratelimit/](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/distributed-ratelimit/) (v3.7 docs branch, no date; "available exclusively in Traefik Hub") ([.sources/proxy-docs-distributed-ratelimit.md](.sources/proxy-docs-distributed-ratelimit.md)).

## Clock and scope

- "It is based on a token bucket implementation." (Hub reference; guide says "the Token Bucket algorithm".)
- Limits apply "throughout your cluster and not only on an individual proxy." State lives in "a persistent KV storage" (Redis). (Hub reference)
- Rate = `limit` / `period`; below 1 req/s use `period` > 1s. `limit` + `period` = refill rate; `burst` = bucket size. (Hub reference)
- Worked examples: `limit 6, period 1m, burst 100` → one token every 10 s when not full, up to 100 "connections in parallel" (Hub reference). `limit 100, period 1s, burst 200` → refill 100 tokens/s, capacity 200, "each IP address will get its own bucket" with `ipStrategy: {}` (guide).
- Redis is required: guide says the middleware "requires you install a Redis() instance"; reference marks `store.redis.endpoints` Required: Yes.
- Single gateway or no sharing wanted → use the plain `RateLimit` middleware instead (guide).

## Options (Hub reference table)

| Field | Default | Meaning on page |
| --- | --- | --- |
| `limit` | 0 | Requests per `period`. "0 means no rate limiting." |
| `period` | 1s | Window for the rate. |
| `burst` | 1 | "Maximum number of requests allowed to go through at the very same moment." |
| `denyOnError` | true | "Forces to return a 429 error if the number of remaining requests accepted cannot be get." `false` lets the request reach the backend. |
| `responseHeaders` | false | Injects `X-RateLimit-Remaining`, `X-RateLimit-Limit`, `X-RateLimit-Period`, `X-RateLimit-Reset`; they show tokens left "after the reservation for the request was made." |
| `store.redis.endpoints` | "" | Required. |
| `store.redis.username` / `password` / `database` | "" / "" / 0 | `database` ignored on Cluster (only 0), with a warning. |
| `store.redis.cluster` | "" | Enable Redis Cluster. |
| `store.redis.sentinel.masterSet` / `username` / `password` | "" | `masterSet` "Required when using Sentinel." |
| `store.redis.tls.caBundle` / `cert` / `key` / `insecureSkipVerify` | "" / "" / "" / false | TLS to Redis. |
| `sourceCriterion.requestHost` / `requestHeaderName` / `ipStrategy.depth` / `ipStrategy.excludedIPs` | false / "" / 0 / blank | Same grouping model as OSS. None set → remote address; several set → error. No `ipv6Subnet` row. |

Redis modes listed: single instance, Redis Cluster, Redis Sentinel (reference and guide).

**Conflict (two official pages, same middleware):** the Proxy-docs copy differs from the Hub reference on: `responseHeaders` lists only Limit / Remaining / Reset (no `X-RateLimit-Period`); TLS CA key is `tls.ca`, not `tls.caBundle`; it has a `store.redis.timeout` row ("dial, read, and write operations"), which the Hub reference shows only in its example (`timeout: 500ms`); `denyOnError` is worded as "deny requests when the rate limit store (Redis) is unavailable" without a status code; `cluster` is `{}` and cannot be combined with Sentinel. Neither page is a source pin. Prefer the Hub reference (product's own site, dated Nov 18, 2026) for field names, and verify against a running Hub before relying on either.

## Not documented on the pages read

Absent from all three pages; do not assume an answer:

- Any local / in-process cache in front of Redis.
- Sync interval or sync mechanism between gateway instances (only "throughout your cluster" + shared KV).
- Redis round trips (or script / command shape) per request.
- `Retry-After` header.
- Value formats or units of the `X-RateLimit-*` headers (e.g. what `Reset` and `Period` contain).
- `burst: 0` behaviour (only the default 1 is given).
- Status code for a request rejected by the limit itself (429 is stated only for `denyOnError`), and whether any delay-then-pass applies (OSS `rateLimit` waits up to a computed maxDelay; nothing equivalent is documented here).
- Redis key layout, TTL, or the default `timeout`.
