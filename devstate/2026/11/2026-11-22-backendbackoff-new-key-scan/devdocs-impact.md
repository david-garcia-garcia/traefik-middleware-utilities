# Devdocs impact
change: backendbackoff-expiry-list

## Units
- Backend backoff — subsystem — `backendbackoff/` (`gate.go`, `allow.go`)

## Findings
- [x] stale-usage  Backend backoff — `std_go_backendbackoff` How-to, Gotcha, and Key files omitted the expiry-ordered list (O(1) new-key, cap 65536, Allow/Report tail move, backwards-clock caveat)
