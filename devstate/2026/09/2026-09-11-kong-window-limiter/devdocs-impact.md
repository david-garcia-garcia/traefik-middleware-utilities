# Devdocs impact
change: add-ratelimit-sliding-window

## Units
- Rate limit window counter — subsystem — `ratelimit/`
- SimpleRedis — subsystem — `simpleredis/`
- Reclaim table — pattern — `Sleep`/`Wake`/`Close` on the limiter

## Findings
- [x] stale-usage  Rate limit window counter — `std_go_ratelimit` Key files pointed at main specs not on disk yet; Sleep flush-then-stop missing from Gotchas
