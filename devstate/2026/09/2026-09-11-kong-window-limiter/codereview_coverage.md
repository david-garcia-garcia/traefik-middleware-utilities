# Test coverage

Ticket job (requirement.md / proposal Why): add a Kong-style sliding-window `ratelimit/` package with exact and buffered Redis sync, proven by fake-TCP unit tests and live Redis/Dragonfly + Yaegi suites.

1. [hard] Critical path untested — `ratelimit/limiter.go:951-954` (`case <-ticker.C: _ = l.flushPending()`) — buffered tests always flush via `Sleep()` (`TestBuffered_TwoClientsShareWithoutLastWriteWins`, live `bufferedTwoClients`); reverting tick-driven flush leaves every test green
   → Take with `sync_rate` at `minSyncRate`, wait one tick without `Sleep`, assert Redis holds the flushed delta
   Status: done
   Argument: TestBuffered_TickerFlushesWithoutSleep (`1979b40`).

2. [hard] Edge case untested — `ratelimit/limiter.go:79-81` (`windowSec < 1` returns error) — spec forbids sub-second windows; `(none)`
   → Assert `Take(..., 500*time.Millisecond)` returns the window error
   Status: done
   Argument: TestTake_SubSecondWindow (`1979b40`).

3. [hard] Edge case untested — `ratelimit/limiter.go:53-55` (positive `sync_rate` below 20 ms floors to `minSyncRate`) — spec names the floor; tests pass `minSyncRate` or `time.Hour` only (`TestClose_StopsTickerAndKeepsRedis`); `(none)`
   → `New(client, time.Millisecond)` then assert the limiter's effective interval is 20 ms (e.g. tick fires within a bounded wait)
   Status: done
   Argument: TestNew_FloorsSyncRateBelow20ms (`1979b40`).

4. [judgement] Happy path only — `ratelimit/limiter.go:794-797` (`Allow` delegates to `Take`) — `TestAllow_IsTake` asserts only the first allow; deny equivalence untested
   → Mirror `TestTake_NThenDeny` through `Allow`, or drop if alias coverage is intentionally minimal
   Status: skipped
   Argument: judgement — Allow is an alias; Take proves deny. TestAllow_IsTake keeps first-allow.
