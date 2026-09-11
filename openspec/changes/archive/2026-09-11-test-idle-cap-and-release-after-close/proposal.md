## Why

Dest already caps idle sockets at eight and closes in-flight sockets on `release` after `Close`, but the tests that look like they prove those branches cannot fail: eight goroutines cannot exceed eight dials, and Get-after-`Close` returns `redis:unreachable` before `borrow`/`release`. A silent idle leak or a `Close` that still repools would stay green.

## What Changes

- Prove idle `len(idle) <= 8` after more than eight overlapping commands, and that excess sockets are closed (not leaked). Do not add a total connection cap or wait queue (perf-01 stays out of scope).
- Prove an in-flight command that started before `Close` still finishes and its socket is closed with `idle` empty.
- Cover `borrow`'s second `closed` check (idle scan done, then `Close` before `dial`) with a nil-checked same-package test hook. Production pool constants and `release`/`borrow` behavior stay as they are.
- Repair `TestConcurrentCommandsStayWithinPool` so the goroutine count can violate the idle assertion. Keep Close-then-Get redial coverage; replace the vacuous final idle assertion with the in-flight-`Close` test that actually calls `release`.
- Live overlap smoke on existing `/redis` and `/dragonfly`: 16 parallel requests, then `CLIENT LIST` (bare, no `TYPE`/`ID`) remaining connections ≤ 8. Reuse the existing probe EVAL (Lua 5.1-safe, KEYS declared). No idle-count export, no host port publish, no probe idle header.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: idle pool SHALL be at most eight after release; in-flight MAY exceed eight; after overlap of more than eight commands, idle ≤ 8 and excess sockets closed. Close SHALL close in-flight sockets on release. Live `/redis` and `/dragonfly` overlap MUST not leave more than eight idle sockets. Sequential reuse and Close-then-Get `redis:unreachable` stay.

## Impact

- `simpleredis/simpleredis_test.go` (hold fake, idle-cap and in-flight-Close tests, repair of tautological tests).
- `simpleredis/simpleredis.go` only for a nil-checked test hook after idle-scan unlock and before the second `closed` check.
- `scripts/integration-tests.Tests.ps1` (`/redis`, `/dragonfly` overlap + `CLIENT LIST`).
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Probe `ServeHTTP` and compose Redis/Dragonfly stay as they are. No `IdleCount` export. No production pool-constant change.
