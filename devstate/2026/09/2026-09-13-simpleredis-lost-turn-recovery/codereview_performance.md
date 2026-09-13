# Performance

1. [hard] Unbounded collection or cache — `simpleredis/pool.go:112` — after a leak, each pool-wait waiter can mint a turn and dial; live sockets grow with concurrent waiters past `PoolSize`
   → Fill and take the recovered turn (or increment `heldSockets`) before unlocking `turnRecoverMu` so a second waiter cannot top up `cap-len` and dial. Grows with concurrent pool-wait waiters (request traffic after leaked turns); `PoolSize` is supposed to be the cap but `heldSockets` is only incremented at dial, after the mutex is released.
   Status: done
   Argument: borrowAfterPoolWait holds turnRecoverMu through recover, take, idle, and dial.
