# Devdocs impact
change: test-idle-cap-and-release-after-close
pin: origin/master (7dc4b05)

## Units
- SimpleRedis — subsystem — `simpleredis/simpleredis.go` (spec `std_go_simpleredis_tcp-session`)

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` Close Gotcha omitted in-flight finish + socket closed on release
