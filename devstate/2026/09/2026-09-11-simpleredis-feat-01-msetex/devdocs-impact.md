# Devdocs impact
change: simpleredis-msetex

## Units
- SimpleRedis — subsystem — `simpleredis/` (`std_go_simpleredis`)

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` How-to/Gotchas omitted integer-0 `redis:issue?` (Expire `:0` success does not apply), pass-through TTL, native→Lua recache, and past-EXAT Get miss
