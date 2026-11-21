# Nitpicks

1. [hard] Clear conditions — `windowcounter/fake_redis_test.go:78` — `serve` GET rewrites a zero hold to `hold = -1` then gates on `if hold != 0` / `if hold > 0` / else `<-release`; the wait body is timed hold vs wait-until-release, not “any non-zero duration”
   → Name those cases (`timedHold` vs `waitUntilRelease`) without stuffing a mode into a negative `time.Duration`
   Status: done
   Argument: `timedHold` vs `waitUntilRelease`; no negative duration sentinel.
2. [hard] Linear coding — `windowcounter/fake_redis_test.go:69` — `serve` GET inlines unlock, entered signal, timer/select, write, relock (the GET-hold lifetime) inside the command switch
   → Extract a method named for that hold (`waitGetHold`); Unlock during the wait lives inside it; `serve` asks that name then writes the reply
   Status: done
   Argument: `waitGetHold` owns the wait; `serve` GET unlocks, calls it, writes the reply, relocks.
