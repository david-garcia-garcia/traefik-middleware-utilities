## Why

A panic between `borrow` and `release` in `exec` permanently drops one in-use-turn token. After `PoolSize` such panics the semaphore is empty; later commands wait `PoolTimeout` and return `redis:unreachable` for the life of the process even when Redis is healthy.

## What Changes

- After a successful `borrow` in `exec`, always return the token when `do` returns or panics. On panic, treat the socket as not reusable (close it, do not idle-pool it).
- Do not recover the panic in `exec`. Let it propagate.
- Prove the invariant in `simpleredis/pool_test.go` with `startStaticRedis`: after `PoolSize` recovered panics, `len(inUseTurns) == cap(inUseTurns)` and a later `borrow` succeeds.
- Do not cap parser allocations, make `freeInUseTurn` non-blocking, wrap `dial` AUTH/SELECT, or recover-and-map to `redis:issue?`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: a panic inside `do` after a successful `borrow` MUST still return the in-use-turn token and MUST NOT recover; the abandoned socket MUST NOT return to idle.

## Impact

- `simpleredis/commands_exec.go` (`exec` attempt-scoped release)
- `simpleredis/pool_test.go` (panic-unwind token conservation)
- Main spec `std_go_simpleredis_tcp-session` after archive
- Handshake `dial` AUTH/SELECT panic remains out of scope (`knowledge/debt/2026-09-12-dial-handshake-panic-leaks-token.md`)
