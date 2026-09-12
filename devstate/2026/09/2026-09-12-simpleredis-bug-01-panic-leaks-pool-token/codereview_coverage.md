# Test coverage

1. [hard] Critical path untested — `simpleredis/commands_exec.go:40` — `doAndRelease` leaves `reusable` false so panic `release` closes the socket (`pool.go:129`); `TestPanicDuringDoReturnsInUseTurn` (`pool_test.go:410`) asserts `len(inUseTurns)==cap` and later `borrow`, not idle empty. Revert of `reusable` false still greens that test.
   → Assert `pooledIdle(redis)==0` after the two recovered panics
   Status: done
   Argument: asserted pooledIdle==0 after the two recovered panics in TestPanicDuringDoReturnsInUseTurn (9aa8aa1).
