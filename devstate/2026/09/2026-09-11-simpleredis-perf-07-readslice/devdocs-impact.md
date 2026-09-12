# Devdocs impact
change: simpleredis-readslice-decode

## Units
- SimpleRedis — subsystem — `simpleredis/` (`knowledge/devdocs/std_go_simpleredis.md`)
- RESP decode — pattern — `simpleredis/simpleredis.go` (`readLine`, `readReply`, `parseLen`); spec `std_go_simpleredis_resp-decode`

## Findings
- [x] missing-packet  RESP decode — no packet; only caller How-to on `std_go_simpleredis`
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` Key files listed tcp-session and resp-commands, not the decode spec
