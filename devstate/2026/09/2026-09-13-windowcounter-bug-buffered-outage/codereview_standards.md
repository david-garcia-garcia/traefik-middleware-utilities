# Standards

1. [hard] Name for the scope — `windowcounter/limiter.go:42` — `lastFlushErr` was retargeted from “returned by buffered Take/Peek” to the skip-Redis outage store (`last failed flush or GET`); Flush drops the GET/outage clause
   → Rename to `lastOutageErr` (and `flushFailedAt` to the same stem)
   Status: done
   Argument: renamed `lastFlushErr` to `lastOutageErr` and `flushFailedAt` to `lastOutageAt`.
