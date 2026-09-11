# Performance

1. [judgement] Unbounded collection or cache — `simpleredis/simpleredis.go:71` — `digests` is keyed by full script body with no max size, TTL, or eviction; grows with distinct `Eval` scripts on one client
   → Cap or evict only if callers pass unbounded unique scripts; tokenbucket, windowcounter, and the probe use one const each
   Status: skipped
   Argument: judgement; no growing reachable path in this tree — Eval callers pass package consts, not unbounded unique scripts.
