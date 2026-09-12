# SimpleRedis dial circuit breaker

IssueKey: 2026-09-12-simpleredis-risk-01-no-context-uncancellable-latency
Size: large
Action: note

## Why this follow-up
After k consecutive dial failures, fail immediately for a cooldown so the Nth concurrent request during an outage is `redis:unreachable` instead of another full retry ladder. Yaegi-safe: atomic last-failure time plus a counter.

## Why it was not taken
The finding ranks the breaker after the three numbered fixes (defaults, overall exec budget, context). This run is bound to those three. A breaker is a new failure-memory policy with a threshold and cooldown this requirement did not specify.

## Risks
Without a breaker, concurrent requests still rediscover a dead Redis independently, each paying the (now shorter) derived budget and holding a pool turn while they dial.
