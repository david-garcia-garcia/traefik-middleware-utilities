# Standards

1. [hard] Leave a trail — `windowcounter/limiter.go:405` — `flushPendingLocked` now copies in-flight snapshots, drops `l.mu` for EVAL, then subtracts flushed delta; the method comment still only restates that the caller holds `l.mu`, and the EVAL/merge loop has no block intro (copy and expire do)
   → Say that job on the method (in-flight mark, unlocked EVAL, subtract flushed delta, second flusher skips) and introduce the EVAL/merge loop
   Status: done
   Argument: method comment names in-flight mark, unlocked EVAL, subtract flushed delta; EVAL/merge loop has a block intro.
