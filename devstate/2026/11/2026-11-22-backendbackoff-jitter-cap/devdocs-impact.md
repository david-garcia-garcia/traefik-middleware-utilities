# Devdocs impact
change: backendbackoff-jitter-below-cap

## Units
- Backend backoff — subsystem — `backendbackoff/` (`cooldownDuration` in `backendbackoff/gate.go`)

## Findings
- [x] stale-usage  Backend backoff — `std_go_backendbackoff` Gotchas omit the live jitter interval at MaxCooldown
