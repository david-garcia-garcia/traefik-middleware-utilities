# Devdocs impact
change: tokenbucket-canceled-request

## Units
- Token bucket — subsystem — `tokenbucket/` (`knowledge/devdocs/std_go_tokenbucket.md`)

## Findings
- [x] stale-usage  Token bucket — `std_go_tokenbucket` How-to omitted 499 / skip-wait as the middleware's job; Gotchas omitted Redis in-flight charge on `Canceled`/`DeadlineExceeded` and that Memory already-done ctx charges nothing
