# Test coverage

1. [hard] Critical path untested — `simpleredis/pool.go:98` — cancelled wait for a pool turn returns `ctx.Err()` and MUST NOT take a turn; `TestGetContextCancelFreesTurnAndDoesNotPool` cancels after the turn is held, `TestGetContextAlreadyCancelledDoesNotSend` returns before `borrow`; `(none)` hits the contended `ctx.Done()` arm
   → Hold PoolSize 1, cancel a waiting GetContext, assert `ctx.Err()` and that the waiter never acquired a turn
   Status: done
   Argument: TestGetContextCancelWhileWaitingForTurn holds PoolSize 1 and cancels the waiter.
