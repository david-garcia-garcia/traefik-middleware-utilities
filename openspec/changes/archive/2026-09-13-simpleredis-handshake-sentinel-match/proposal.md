## Why

Under Yaegi, a handshake AUTH/SELECT failure is wrapped in a package-local struct whose `Unwrap` compiled `errors.Is` cannot see. Callers that follow the documented matcher contract take the wrong fail-open or fail-closed branch on restart, failover, LOADING, and max-clients — the paths those matchers exist to catch.

## What Changes

- Stop carrying the do-not-retry mark in the error type. `dial` returns the inner error unchanged plus `handshakeFailed bool`; `borrow` forwards it; `exec` passes it to `shouldRetry`.
- Delete `handshakeFailure`, `isHandshakeFailure`, and `Unwrap`. Do not use `errors.As`. Do not match `Error()` text.
- Preserve: AUTH/SELECT EOF, AUTH `-LOADING …`, AUTH `-ERR max number of clients reached` are not retried (exactly 1 TCP accept). Still retried: TCP dial refuse before handshake; `LOADING ` after a successful handshake.
- Port the two Yaegi `TestBug*` regressions into the default suite (interpreter assertion plus compiled control). Add interpreted matcher coverage for package error paths. Rewrite `TestShouldRetryHandshakeFailureIsFalse` onto the new bool.
- Tighten handshake THEN lines so peer-close matches `IsUnreachable` and AUTH-class matches `errors.Is(..., ErrNoAuth)` under both compiled and interpreted callers.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: handshake AUTH/SELECT failures SHALL surface the inner sentinel (peer-close is `IsUnreachable`; AUTH-class is `ErrNoAuth`) and MUST NOT open a second TCP connection. The do-not-retry mark MUST NOT live in the error type.
- `std_go_simpleredis_resp-commands`: interpreted callers SHALL match handshake AUTH EOF with `IsUnreachable` and AUTH WRONGPASS with `errors.Is(..., ErrNoAuth)`, not only a `%w` wrap of `ErrMiss`.

## Impact

- `simpleredis/pool.go` (`dial` triple return; delete `handshakeFailure`)
- `simpleredis/commands_exec.go` (`borrow` bool; `shouldRetry(err, handshakeFailed)`)
- `simpleredis/yaegi_test.go` (ported handshake matcher tests + extra package-error matcher coverage)
- `simpleredis/pool_test.go` (existing no-second-connection tests stay green)
- `simpleredis/commands_exec_test.go` (rewrite `TestShouldRetryHandshakeFailureIsFalse`)
- `simpleredis/errors_test.go` (`shouldRetry` call sites gain the bool)
- Main specs `openspec/specs/std_go_simpleredis_tcp-session/spec.md` and `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive
