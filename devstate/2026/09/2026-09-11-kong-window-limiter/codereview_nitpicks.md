# Nitpicks

1. [hard] Symmetry and consistency — `ratelimit/limiter.go:239` — `stopFlushLocked` unlocks `l.mu`, waits on `l.wg`, then re-locks; sibling `startFlushLocked`, `windowLocked`, and `bufferedCountLocked` keep the mutex held for their whole body
   → Split stop (while locked) from wait (caller unlocks first), or drop the `Locked` suffix on the unit that releases the lock so all mutex helpers share one contract
   Status: done
   Argument: `takeFlushTickerLocked` holds mu; `stopFlushAndWait` waits outside (`1979b40`).

2. [hard] Name for the scope — `ratelimit/limiter.go:239` — `stopFlushLocked` reads like the other `*Locked` helpers but its job includes unlock-wait-relock, which the name omits
   → Rename to spell the full job (e.g. stop flush then wait for goroutine outside the lock) or use a name that does not imply the standard locked-helper contract
   Status: done
   Argument: renamed off `stopFlushLocked` (`1979b40`).
