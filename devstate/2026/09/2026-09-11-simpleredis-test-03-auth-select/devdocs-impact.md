# Devdocs impact
change: simpleredis-test-03-auth-select

## Units
- SimpleRedis — subsystem — `simpleredis/` (`std_go_simpleredis_tcp-session`, `std_go_simpleredis_resp-commands`)

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` How-to/Gotcha/Key files missed AUTH-before-SELECT, empty-field skip, `live_test.go`, and e2e failure PathPrefix
- [ ] language-gap  Handshake — `std_go_simpleredis` has How-to and Gotcha for dial AUTH/SELECT, no Language term
  Skip: explore nickname; product owner is `dial`; do not invent a term.
