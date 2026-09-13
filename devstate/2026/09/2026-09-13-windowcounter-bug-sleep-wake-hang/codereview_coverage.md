# Test coverage

1. [hard] Edge case untested — `windowcounter/limiter.go:363` — `stopFlushAndWait` sets `stopping` then, when `takeFlushTickerLocked` returns nil, clears it before return (design: Wake after an already-stopped Sleep must still start). `TestWake_StartsTickerAfterSleep` Sleeps once while the ticker from `New` is running (wait arm). `TestClose_StopsTickerAndKeepsRedis` then Wakes while `closed`. `(none)` fails if that nil-arm clear is reverted.
   → Assert a second Sleep (ticker already nil) then Wake starts the ticker (`stop != nil`)
   Status: done
   Argument: TestWake_StartsTickerAfterSleep now Sleeps twice then Wake; nil-arm clear would leave stopping true and fail.
