# Devdocs impact
change: simpleredis-incr-eval

## Units
- SimpleRedis — subsystem — `simpleredis/`

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` How-to still says first Set/Get only; Pattern snippet has no Incr/Eval; Gotchas omit that Incr does not refresh TTL and Expire `:0` is success
