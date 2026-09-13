# Dead

1. [hard] Leftover production path — `windowcounter/limiter.go:44` — `lastRedisOK` has no production reader after `bufferedOutageErrorLocked` (the missed-`sync_rate` probe that compared `now.Sub(lastRedisOK)` to `syncRate`) was deleted. Definition plus writes only: `l.lastRedisOK = l.now()` at `peekCountLocked` `:231`, `windowLocked` `:250` and `:264`, `bufferedCountLocked` `:285`, `flushPendingLocked` `:435`. Grep `lastRedisOK`: no `*_test.go` hits; remaining hits are openspec/devstate from the prior probe ticket.
   → Delete `lastRedisOK` and the five `l.lastRedisOK = l.now()` stores
   Status: done
   Argument: deleted `lastRedisOK` and its five stores.
