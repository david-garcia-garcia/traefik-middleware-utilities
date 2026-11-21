# Performance

1. [judgement] Quadratic work on a growing set — `simpleredis/pool.go:171` — every abandon linearly scans `turnWaiters` and every grant (`pool.go:93`) shifts the whole slice with `copy`, both under `inUseMu`; the queue has no length cap, so draining or timing out a backlog of n waiters costs O(n²) under the one lock give-backs also need
   Quote:
   ```go
   // freeInUseTurn (pool.go:91-95), every give-back with a waiter queued
   oldest := sr.turnWaiters[0]
   copy(sr.turnWaiters, sr.turnWaiters[1:])
   // abandon (pool.go:171-179), every PoolTimeout or ctx exit, and again on a miss when it races a grant
   for i, queued := range sr.turnWaiters {
       if queued == waiter {
           copy(sr.turnWaiters[i:], sr.turnWaiters[i+1:])
   ```
   Note:
   ```text
   What grows: n = callers arriving past PoolSize x min(PoolTimeout, caller deadline); default PoolTimeout is
   200ms (config.go:16), and nothing caps len(turnWaiters). The old channel semaphore queued in the runtime at
   O(1) per wake. Under Yaegi the abandon `range` is interpreted per element; the front `copy` is a native memmove.
   An abandon that loses the race to a grant (or the second of timer/ctx) scans the full queue and finds nothing.
   Judgement, not hard: PoolTimeout bounds how long each waiter stays, timeouts mostly hit near the head, and
   no backlog size was measured, so the real cost at saturation is unknown.
   Leak check passed: poolTimer.Stop() and stopCtxAbandon() run after <-waiter.ready on every exit.
   ```
   Fix: Pop the head by index (head offset or ring) instead of `copy`, and have abandon mark the waiter abandoned so the grant pops past it, compacting when abandoned slots pass half the queue; keep it inline with no new method
   Status: skipped
   Argument: Judgement: same as Security 1. Each waiter lives at most PoolTimeout, abandons mostly hit near the head, and no saturated backlog was measured; a head offset plus tombstones is a design change, not applied unattended.
