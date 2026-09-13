## Why

Dest `exec` releases a borrowed socket without `defer`. A panic between `borrow` and `release` loses that in-use turn forever and leaks the socket fd. After `PoolSize` recovered panics (Traefik recovers the request) every later command is `redis:unreachable` against a healthy Redis with zero open sockets. PR #29 closed believing a deferred `release` would not run under Yaegi; a probe on Yaegi v0.16.1 ran the deferred restore for an explicit interpreted panic, the interpreter `errors.As` panic, and a nil-map write.

## What Changes

- `exec` moves the inline `do` + `release` into `runOnConn` with a deferred `release`. `reusable` starts false (covers "do said not reusable" and "do never returned") so a panic closes the socket and returns the turn.
- `borrow` takes a `handedOff` flag and defers `freeInUseTurn()` on every path that does not hand a socket to the caller. The two explicit error-path `freeInUseTurn()` calls are removed so they do not double-free.
- Permanent tests: panic-in-`do` via a panicking `io.Writer` (`simpleredis/panic_safety_test.go`); Yaegi defer-on-panic probe (`simpleredis/yaegi_defer_test.go`). No `zz_`, `proto`, or `scratch` names.
- Stale `do` comment in `resp.go` is updated. Tcp-session spec gains the panic-returns-turn guarantee. Usage packet gains a Yaegi-defer-runs gotcha.
- Do not add leak-detection or turn-refill machinery (`heldSockets`, `LostTurns`, `borrowAfterPoolWait`, …). Do not edit `simpleredis/BUGS.md`. Do not merge `origin/proto-defer-net` wholesale.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: an in-use turn is returned and the socket closed even when the command panics between borrow and release.

## Impact

- `simpleredis/commands_exec.go` (`runOnConn`).
- `simpleredis/pool.go` (`borrow` `handedOff`).
- `simpleredis/resp.go` (`do` comment).
- `simpleredis/panic_safety_test.go`, `simpleredis/yaegi_defer_test.go`.
- `knowledge/devdocs/std_go_simpleredis.md` (Yaegi defer gotcha).
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Expected later mechanical conflict with OPEN #67 on `borrow`.
