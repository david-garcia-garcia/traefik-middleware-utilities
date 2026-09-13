# Test coverage

1. [hard] Critical path untested — `windowcounter/limiter.go:244` — share-refresh GET is gated by `localDelta == 0 && !skipRedisContactLocked()`; this change added `skipRedisContactLocked` so a stored `lastFlushErr` skips Redis. Spec: while that error is stored, Take/Peek MUST NOT GET to refresh the share. Every lock test either has `localDelta > 0` (`TestTake_BufferedPendingDeltaOutage`, `TestTake_BufferedSleepStoresFlushError`, `TestRepro_BufferedTakeHidesRedisOutage`) or `lastFlushErr == nil` on the first Take after kill (`TestTake_BufferedFlushThenKillFailsClosed`). `getCallCount` after `Kill` stays 0 even when the client still GETs (fake never serves). Reverting `skipRedisContactLocked` leaves those tests green.
   → After `lastFlushErr` is stored with `localDelta == 0` (Peek after flush+kill, then Take; fake still accepting so GET would increment), assert Take is `err=nil` and GET count does not rise
   Status: done
   Argument: added `TestTake_BufferedStoredOutageSkipsShareRefreshGET` (stored outage, `localDelta==0`, fake still serving; GET count must not rise).
2. [judgement] Happy path only — `windowcounter/limiter.go:256` — first sight of a *current* window key while Redis is down seeds 0 (`skipRedisContactLocked` or GET error → `seedEmptyWindowLocked`). Tests only Take/Peek the key that was already seeded while Redis was up. Previous-key first sight is hit from Peek after Sleep deletes `expireAt==0` (`TestPeek_BufferedEmptyFlushThenKill`); current-key first sight (new opaque key or rolled window) is not.
   → Take a never-seen key after outage and assert `err=nil` and admit from 0 until `limit`
   Status: skipped
   Argument: judgement — ticket job already proven by `takeUntilLocalDeny` / `peekNilError`; first-sight current-key outage is extra.
