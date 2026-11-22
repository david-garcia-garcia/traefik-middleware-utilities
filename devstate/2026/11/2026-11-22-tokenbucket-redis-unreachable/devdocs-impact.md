# Devdocs impact
change: tokenbucket-redis-unreachable

## Units
- Token bucket — subsystem — `tokenbucket/` / `openspec/specs/std_go_tokenbucket_allow`
- SimpleRedis — subsystem — `simpleredis/` (sentinels; unchanged this change)

## Findings
- [x] stale-usage  Token bucket — `std_go_tokenbucket` How-to matches `IsUnreachable` against `ErrTimeout` (false); omits that fail-open/fail-closed is the middleware's and that `Allowed=false` on a Redis error is the zero Status, not a deny
