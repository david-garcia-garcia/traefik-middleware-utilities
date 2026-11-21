# Devdocs impact
change: simpleredis-leftover-reply-destroy

## Units
- SimpleRedis — subsystem — `simpleredis/` / `knowledge/devdocs/std_go_simpleredis.md`
- RESP decode — pattern — `knowledge/devdocs/std_go_simpleredis_resp-decode.md`

## Findings
- [x] language-gap  Reply boundary — `std_go_simpleredis` had How-to, no Language term for leftover unread RESP
- [x] stale-usage  leftover destroy — `std_go_simpleredis` Gotchas implied a clean parse is always reusable
- none for RESP decode — leftover reuse is session ownership, not ReadSlice decode
