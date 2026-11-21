# Security

1. [hard] Fail-open on deny — `windowcounter/limiter.go:184` — GET/flush error is stored (`rememberOutageLocked`) and buffered Take/Peek return nil with local admit; dest returned that error (and `allowed=false` on GET failure)
   → Fail closed: return the Redis error from buffered Take/Peek on GET or flush failure
   Status: skipped
   Argument: Desired 4 and deviations.md require buffered Take/Peek to keep the per-node cap with `err=nil`; returning the Redis error would restore dest fail-closed.
