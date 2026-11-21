# Devdocs impact
change: simpleredis-stale-pooled-socket-retry

## Units
- SimpleRedis — subsystem — `knowledge/devdocs/std_go_simpleredis.md` / `simpleredis/commands_exec.go`

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` Gotcha still describes one-socket peer-close retry and omits skip-idle recovery of a full idle vintage
