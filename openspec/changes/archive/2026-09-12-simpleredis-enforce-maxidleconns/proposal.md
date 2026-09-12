## Why

`Config.MaxIdleConns` is documented as how many unused sockets `release` will keep, but DestBranch `release` also requires `live >= liveCap()` while this socket still holds its in-use turn. Operators who set a small idle cap under a larger `PoolSize` keep nearly a full pool of resting sockets.

## What Changes

- Trim unused sockets on the idle list alone: close when the client is closed or `len(idleConns) >= MaxIdleConns`. Drop the `inUse` / `live` term from `release`. Keep `liveCap()` for `PoolSize()`.
- Replace `TestReleaseKeepsSocketWhenLiveUnderCap` so it no longer requires idle above the cap. Prove sequential `borrow`/`release` for PoolSize/MaxIdleConns `8/2`, `16/1`, `2/8`, `8/8` with `len(idleConns) == min(MaxIdleConns, PoolSize)`, concurrent quiesce at that cap, and fake still-open sockets via `waitOpenSocketsEqual`.
- Fold `std_go_simpleredis_tcp-session`: unused sockets SHALL be at most `MaxIdleConns`. Remove `release` MUST NOT close solely because idle is full while live is under `PoolSize`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: `release` closes a reusable socket when the idle list is already at `MaxIdleConns`, even while live sockets are under `PoolSize`. Sequential and concurrent proof MUST show idle at `min(MaxIdleConns, PoolSize)` and that excess sockets are closed on the fake.

## Impact

- `simpleredis/pool.go` `release` (and its comment).
- `simpleredis/pool_test.go` (`TestReleaseKeepsSocketWhenLiveUnderCap` and new idle-cap table).
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Usage `knowledge/devdocs/std_go_simpleredis.md` already says idle trim is `MaxIdleConns`; confirm after apply.
- Out of scope: idle reaper, clamping `MaxIdleConns` to `PoolSize`, `PoolSize` semantics, go-redis `0` = unlimited, other `simpleredisfixes2/` files.
