# Devdocs impact
change: simpleredis-configurable-timeouts

## Units
- SimpleRedis — subsystem — `simpleredis/simpleredis.go`; spec `std_go_simpleredis_tcp-session`

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` How-to live-skip sentence inverts `-short`; Options fields `DialTimeout` / `IdleTimeout` unnamed; Gotcha still says only `Init` does not dial
