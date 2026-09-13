# Devdocs impact
change: simpleredis-eval-null-bulk-not-miss

## Units
- SimpleRedis — subsystem — `simpleredis/`, `knowledge/devdocs/std_go_simpleredis.md`
- RESP decode — pattern — `simpleredis/resp.go`, `knowledge/devdocs/std_go_simpleredis_resp-decode.md`

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` How-to Eval and gotcha now say top-level `$-1` is a nil slot; Get maps miss; Eval false is not `redis:miss`; `$0` is not miss
- [x] stale-usage  RESP decode — `std_go_simpleredis_resp-decode` parseLen optional minus is a nil slot / negative array, not miss for every verb
