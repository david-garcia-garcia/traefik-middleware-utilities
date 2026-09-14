# Devdocs impact
change: simpleredis-eval-fallback-hop-comments

## Units
- SimpleRedis — subsystem — `simpleredis/` (`Eval`, `MSetEX`, `MSetEXAt`, `msetex`)

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` How-to and Gotcha state one overall wait per public verb; Eval NOSCRIPT and MSetEX unknown-command each bind a fresh exec budget per hop
