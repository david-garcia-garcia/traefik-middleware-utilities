# Standards

1. [hard] Leave a trail — `tokenbucket/repro_hunt_clock_last_backward_test.go:12` — job comment states dest as present fact (`consumeOne persists nowMicro as last even when nowMicro < last`; `Memory samples now() outside the mutex`) after persist-max and lock-order landed
   → Rewrite the comment as what the test proves now (last does not rewind; Memory samples under the mutex)
   Status: done
   Argument: rewrote the job comment to what the test proves now (persist-max, now under mu, Lua persist-max).
