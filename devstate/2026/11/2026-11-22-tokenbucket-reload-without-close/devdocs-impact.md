# Devdocs impact
change: tokenbucket-reload-without-close

## Units
- SimpleRedis — subsystem — `simpleredis/` (`New`, `Close`, pool); spec owner `std_go_simpleredis_tcp-session`
- Token bucket — subsystem — `tokenbucket/redis.go` (`NewRedis`, `Allow`)

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis.md` Gotcha only covered reachable idle-until-Close; How-to did not say a Traefik reload drop without Close releases sockets after GC
