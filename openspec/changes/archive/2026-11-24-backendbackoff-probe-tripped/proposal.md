## Why

A caller that logs backoff transitions (one line when a key trips, one when a probe closes it) cannot see when the gate drops an idle key. After TTL without an Allow, the next Allow recreates the key CLOSED with full credit, and nothing tells the caller. The caller still believes it is backing off, so the next trip logs nothing. Found in crowdsec-bouncer-traefik-plugin: after a quiet minute, a second WAF outage produced no "backoff started" warning. The only workaround available to callers was to copy the TTL and mirror which calls refresh the idle deadline, which ties them to internals of this package.

## What Changes

- `Permit.Probe() bool`: true when Allow admitted the attempt as the HALF-OPEN probe (OPEN to HALF-OPEN, or taking over an expired probe lease), false for CLOSED admits and the zero `Permit`.
- `Gate.Tripped(key string) (bool, error)`: whether the key is OPEN or HALF-OPEN. A missing or idle-expired key is not tripped. Read-only: no key creation, no state change, no idle deadline refresh. `errClosed` after Close.
- Both are additive. `Allow` and `Report` keep their signatures and behavior.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_backendbackoff_allow`: permits say whether they are the probe; `Tripped` reports the key's backoff state.

## Impact

- `backendbackoff/gate.go` (`Permit.probe`, `Probe`), `backendbackoff/allow.go` (`Allow` marks probe permits, `Tripped`).
- Tests: `backendbackoff/zzz_tripped_test.go`, `TestYaegi_TrippedThenProbe` in `zzz_gate_yaegi_test.go`.
- Docs: `README.md`, `knowledge/devdocs/std_go_backendbackoff.md`.
