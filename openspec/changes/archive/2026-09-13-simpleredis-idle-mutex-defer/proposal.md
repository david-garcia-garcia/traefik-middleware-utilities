## Why

Dest `release` still unlocks `idleConnsMu` by hand on two exit paths. A panic inside that keep-or-close section leaves the mutex locked for the process lifetime. Traefik recovers the request, so every later `borrow`, `release`, and `Close` wait forever while stuck goroutines accumulate. #71 already proved interpreted `defer` runs on panic under Yaegi v0.16.1; this change extends that to the remaining hand-unwound resource.

## What Changes

- Extract the locked keep-or-close decision in `release` into `parkIdleConn`. That method locks, `defer`s unlock, decides, appends on success, and returns whether the socket was parked. `release` closes and `freeInUseTurn`s after it returns. A top-of-`release` `defer Unlock()` is a regression (mutex held across `conn.close()` and the turn-channel send).
- Close decision, `lastUsed` stamp, `freeInUseTurn` last, and `OverFrees` (exactly one free per `release`) stay the same.
- Convert the two trivial `groupWriteMu` unlocks in `cachedGroupWrite` / `storeGroupWrite` to `defer`.
- Replace `simpleredis/BUGS.md` section 2's stale "exec releases without defer" record with the measured Yaegi-defer finding and a pointer at `simpleredis/yaegi_defer_test.go`.
- Do not change `Close`, `takeIdleConn`'s stale-after-lock close, `errors.Is` vs `net.Error`, `context.AfterFunc`, `math/rand` jitter, or the `OverFrees` counter itself.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: a panic inside the idle keep-or-close decision still unlocks the idle-list mutex; close and the in-use-turn return still happen after that unlock; the keep-or-close arithmetic still sees this socket's turn as held.

## Impact

- `simpleredis/pool.go` (`release`, new `parkIdleConn`).
- `simpleredis/commands_msetex.go` (`cachedGroupWrite`, `storeGroupWrite`).
- `simpleredis/BUGS.md` section 2.
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Stacked on #71. Expected mechanical conflict with OPEN `2026-09-13-simpleredis-close-abandoned-socket` on `release`.
