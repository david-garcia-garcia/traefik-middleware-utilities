# Deviations

- [x] taken  Hour TTL in `newBurst1Delay0` instead of dest `testTTL` (2s)
  Asked: copy helpers that use `testTTL`
  Instead: `time.Hour` so t0..t2 does not expire the bucket
  Owner: `tokenbucket/repro_hunt_clock_last_backward_test.go`
  Why: dest `testTTL` is 2s and the sequence spans 10s; expire would admit a new burst and hide rewind
  By: implement
  Requester: not asked

- [x] taken  goroutine subtest does not wait inside `now()`
  Asked: copy the dest handshake that waits in `now()` between sample and lock
  Instead: overlap two Allows without parking in `now()`; dest FAIL was recorded with the handshake before `now()` moved under `mu`
  Owner: `tokenbucket/repro_hunt_clock_last_backward_test.go`
  Why: after `now()` runs under `mu`, waiting in `now()` deadlocks
  By: implement
  Requester: not asked
