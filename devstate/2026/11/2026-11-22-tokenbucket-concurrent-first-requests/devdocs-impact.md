# Devdocs impact
change: tokenbucket-concurrent-first-requests

## Units
- Token bucket — subsystem — `tokenbucket/` / `openspec/specs/std_go_tokenbucket_allow` / `std_go_tokenbucket_lua-eval`

## Findings
- [x] stale-usage  Token bucket — `std_go_tokenbucket` How-to live-proof list omitted `traefik_issue14328_e2e_test.go`; Gotchas did not name concurrent first Allows sharing one bucket (mutex / one Eval, admits ≤ burst)
