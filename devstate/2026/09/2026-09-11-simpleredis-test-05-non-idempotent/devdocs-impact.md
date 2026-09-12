# Devdocs impact
change: retry-only-idempotent-commands

## Units
- SimpleRedis — subsystem — `simpleredis/simpleredis.go` (`std_go_simpleredis`)

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` already has the retry-by-verb Gotcha matching the apply
