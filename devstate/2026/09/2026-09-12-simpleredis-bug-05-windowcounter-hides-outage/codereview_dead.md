# Dead

1. [hard] Dead branch or constant — `windowcounter/limiter.go:39` — `flushFailedAt` (`flushFailedAt time.Time // when lastFlushErr was stored`) is stored on flush/probe failure and never read. Grep `flushFailedAt`: definition plus writes at `limiter.go:410` and `:440`; no production reader; tests zero hits (remaining hits are openspec/devstate).
   → Delete the field and the two `l.flushFailedAt = l.now()` stores
   Status: skipped
   Argument: requirement Desired names store flushFailedAt with lastFlushErr; LastFlushError() is out of scope so this run has no reader. Not deleted.
