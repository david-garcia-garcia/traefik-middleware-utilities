## Why

On DestBranch, `IdleTimeout` only gates reuse of the newest idle tail. Older sockets at the head stay established while traffic recycles that tail, so a later burst reuses a cold (often already server-closed) connection and pays a failed command plus a redial.

## What Changes

- On borrow, sweep the whole idle list: close sockets older than `IdleTimeout` outside the lock; reuse the newest survivor (LIFO). Do not start a reaper goroutine.
- Prove on the fake: two idle sockets of differing ages, one borrow, the aged head sockets are closed (`openSockets`, not only `len(idleConns)`). `runtime.NumGoroutine()` is unchanged across `New`.
- Spec: an idle socket older than `IdleTimeout` SHALL be closed when a later command borrows, even if a younger tail is reused. Do not require close with no later borrow.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: an idle connection older than `IdleTimeout` SHALL not be reused **and SHALL be closed** on the next borrow, including when it sits behind a younger tail. Sequential reuse, live cap, and Close drain stay.

## Impact

- `simpleredis/pool.go` (`takeIdleConn`)
- `simpleredis/pool_test.go` (two-age head close; goroutine count across `New`)
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive
- Usage gotcha on `knowledge/devdocs/std_go_simpleredis.md` after apply
- No `New` ticker. No `reclaim` wiring. No bug-03 `MaxIdleConns` change. No peel-on-release. No live Redis/Dragonfly idle-head tests.
