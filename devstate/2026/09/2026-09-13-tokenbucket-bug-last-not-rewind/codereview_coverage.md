# Test coverage

1. [hard] Assertion does not prove the job — `tokenbucket/memory.go:59-62` — Memory samples `now` after `mu.Lock()` so lock order is clock order; `goroutines_stale_samples_before_fresh_lock` (`tokenbucket/repro_hunt_clock_last_backward_test.go:43-83`) closes `started` on the first `now()` then only asserts a later Allow at t2 is denied. After `now()` runs under `mu`, that close happens while the first Allow still holds the lock, so the overlap is t1 then t2 in lock order (same deny as sequential t1,t2). Reverting `now()` to before `mu.Lock()` leaves `consumeOne` persist-max, so last still cannot rewind and the deny stays green. Dest FAIL needed a wait between sample and lock (`devstate/2026/09/2026-09-13-tokenbucket-bug-last-not-rewind/deviations.md`).
   → Park a t1 sample outside the mutex until a t2 consume has stored last, without waiting inside `now()` while `mu` is held, and assert an outcome that fails if `now()` is sampled before Lock
   Status: done
   Argument: first now() waits; if the second Allow finishes then, fail (now sampled before mu). After 500ms still blocked, proceed so the test does not deadlock.
