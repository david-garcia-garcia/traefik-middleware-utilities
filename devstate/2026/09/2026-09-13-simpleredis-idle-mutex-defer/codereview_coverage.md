# Test coverage

1. [hard] Edge case untested — `simpleredis/pool.go:185` — full idle at live cap returns false (close, not park); `TestReleaseKeepsSocketWhenLiveUnderCap` only asserts park when live is under cap; `(none)` asserts the socket is closed
   → Assert a reusable release while idle is already at MaxIdleConns and live is at PoolSize closes the socket, returns the turn, and OverFrees stays 0
   Status: done
   Argument: added `TestReleaseClosesWhenIdleFullAtLiveCap` in `simpleredis/pool_test.go`.
