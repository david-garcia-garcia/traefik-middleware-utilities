# Consumer should share one Memory limiter per middleware name

IssueKey: 2026-11-22-tokenbucket-memory-instance-scope
Size: large
Action: note

## Why this follow-up

Traefik issue [#13706](https://github.com/traefik/traefik/issues/13706) (PRs [#13823](https://github.com/traefik/traefik/pull/13823), [#13707](https://github.com/traefik/traefik/pull/13707)): N routers that reference one RateLimit middleware get N in-memory budgets per source, while Redis keys by middleware and shares one. This library matches that split by construction: each `*Memory` is its own key space; Redis limiters on the same key share server state. A middleware that wants one budget per middleware name (Memory and Redis agreeing) must share one `*Memory` (for example `reclaim` keyed by middleware name) and use the same key prefix on Redis. That wiring is the consumer's, not `tokenbucket`'s.

## Why it was not taken

The binding design for this ticket is documentation and contract tests only. No limiter behavior change. A process-wide Memory map or a Traefik-style `scope` option would be a new product ask and would belong in the middleware that builds the limiter.

## Risks

A consumer that calls `NewMemory` once per router keeps N budgets per source in memory and one budget on Redis for the same configured limit. Switching store then tightens the limit without a config change. Traefik measured limits tens to a hundred times looser than configured when many routers share one middleware.
