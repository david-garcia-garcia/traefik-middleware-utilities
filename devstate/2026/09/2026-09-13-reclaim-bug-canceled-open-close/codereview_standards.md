# Standards

1. [hard] Leave a trail — `reclaim/table.go:300` — `drop` still says this goroutine waits grace, close, and dispose; this change starts `waitGraceOrWake` on another goroutine for positive grace, and the Table overview at 52-55 still says one goroutine writes those lines
   ```
   // drop removes one holder. When it was the last one, this goroutine ends the incarnation: sleep,
   // orphan, grace, close, dispose — in that order, so those lines cannot be reordered.
   ...
   go t.waitGraceOrWake(key, incarnation, woken, grace)
   ```
   → Say `drop` sleeps and orphans, then `waitGraceOrWake` waits grace and expires; update the Table overview to match
   Status: done
   Argument: drop and Table overview now name waitGraceOrWake (`341db7d`).
