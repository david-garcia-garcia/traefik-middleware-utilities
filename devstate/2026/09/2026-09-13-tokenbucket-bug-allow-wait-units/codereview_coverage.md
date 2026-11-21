# Test coverage

1. [hard] Critical path untested — `tokenbucket/redis.go:76` — Redis.Allow now admits with `waitMicro <= float64(maxDelay.Microseconds())` (deny/public contract this change retargeted). New tests only call `NewMemory`; existing Redis Allow tests use rates where dest Duration admit and microseconds admit agree, so they stay green if `redis.go` is reverted to `allowedFromWait`. Production: `allowed := waitMicro <= float64(r.clock.maxDelay.Microseconds())`. Test: `(none)`.
   → Assert Redis.Allow denies on one dest-failing split (1500ns / maxDelay 0 / overflow) and that a following Allow is not a stacked consume
   Status: done
   Argument: added TestRedis_MaxDelayTruncationFailOpen (47972db).
