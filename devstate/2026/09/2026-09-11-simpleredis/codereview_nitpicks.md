# Nitpicks

1. [hard] Name for the scope — `simpleredis/simpleredis.go:234` — `do` is a vague verb; the body sets a deadline, writes a RESP command, reads the reply, and maps dirty IO
   → Name the job (`runCommand` / `roundTripOnConn`)
   Status: skipped
   Argument: copied client @ 6548da47; renaming unexported do would diverge from the pinned source. Bound the ask.
2. [hard] Symmetry and consistency — `Test-Integration.ps1:23` — `Test-RedisHealth` hardcodes `Waiting for Redis...` / `Redis is ready` while sibling `Test-ServiceHealth` uses `$ServiceName` for the same wait/ready role (`$ServiceName` is only on the Redis timeout error)
   → Use `$ServiceName` in the Redis wait and ready steps
   Status: done
   Argument: wait/ready steps use `$ServiceName`.
3. [hard] Clear conditions — `simpleredis/simpleredis.go:139` — `if err == nil || reusable || !reused || err == errTimeout` is the complement of “dead pooled conn”; the retry body is that case
   → `if reused && !reusable && err != nil && err != errTimeout` (or `deadPooledConn(...)`) then retry
   Status: skipped
   Argument: copied client retry membership; rewriting the predicate would reshape source control flow. Bound the ask.
