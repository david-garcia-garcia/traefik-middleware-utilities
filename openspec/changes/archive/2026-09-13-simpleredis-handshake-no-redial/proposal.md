## Why

Handshake AUTH or SELECT failure on dest still redials: `dial` returns the `do` error unchanged, and `exec` retries EOF (`redis:unreachable`), `LOADING …`, and `ERR max number of clients reached`. The tcp-session spec already says a handshake failure surfaces one error and MUST NOT open a second TCP connection. WRONGPASS / SELECT 99 already stay at one accept because those replies are not retryable.

## What Changes

- Land four compiled repros under default `go test -short ./simpleredis/` that fail on dest: AUTH close-no-reply, AUTH OK then SELECT close-no-reply, AUTH `-LOADING Redis is loading the dataset in memory`, AUTH `-ERR max number of clients reached`. Assert one TCP accept with `MaxRetries: 1`.
- After those tests exist, mark AUTH/SELECT failures at `dial` return so `exec` does not retry them. Keep inner `Error()` / `Unwrap()`. TCP `DialContext` refuse stays unmarked and still retries. GET LOADING still retries.
- Add observable scenarios on the existing handshake requirement for those four dest paths. Keep WRONGPASS / SELECT 99 / GET LOADING coverage.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: handshake AUTH or SELECT failure (including peer close with no reply, `LOADING …`, and max-clients) MUST NOT open a second TCP connection for that command.

## Impact

- `simpleredis/pool.go` (`dial` AUTH/SELECT return mark)
- `simpleredis/commands_exec.go` (`shouldRetry` rejects the mark)
- `simpleredis/pool_test.go` (four repros; keep WRONGPASS / SELECT 99)
- `simpleredis/commands_exec_test.go` (GET LOADING still retries)
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive
