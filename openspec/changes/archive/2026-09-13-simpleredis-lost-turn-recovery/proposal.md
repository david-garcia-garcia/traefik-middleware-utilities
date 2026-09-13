## Why

A panic between `borrow` and `release` drops one in-use turn forever. After `PoolSize` recovered panics the client returns `redis:unreachable` with a healthy Redis and zero owned sockets. Traefik recovers the request, so the process stays up and the client dies quietly.

## What Changes

- When `borrow` would return a pool-wait error, if the client owns zero sockets (idle empty and no still-running command holds a socket), refill the in-use-turn channel to `PoolSize` and count the restored tokens on `LostTurns()`.
- Recovery MUST NOT fire while sockets are genuinely live and busy. `PoolSize` stays the frozen live-socket cap.
- Export read-only `LostTurns()` next to `OverFrees()`.
- Port `TestBugLostInUseTurnBricksPoolPermanently` into the default suite so it passes; add a busy-pool test that still returns `redis:unreachable` and never dials past the cap; add a `LostTurns()` assertion.
- Do not `defer sr.release` (PR 29 discarded that). Do not start a reaper. Do not edit `simpleredis/BUGS.md` or `simpleredis/resp.go`. Do not rewrite handshake error classification.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: wait-not-dial is backpressure only when the client owns live sockets at `PoolSize`. A leaked turn with zero owned sockets SHALL refill. `LostTurns()` SHALL be readable next to `OverFrees()`.

## Impact

- `simpleredis/pool.go` (borrow recovery, `heldSockets`).
- `simpleredis/commands_exec.go` (`heldSockets` around `do`, not deferred `release`).
- `simpleredis/simpleredis.go` (`LostTurns()`).
- `simpleredis/` default-suite tests (port of the bugrepro plus busy-pool and `LostTurns()`).
- Usage packet `knowledge/devdocs/std_go_simpleredis.md`.
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
