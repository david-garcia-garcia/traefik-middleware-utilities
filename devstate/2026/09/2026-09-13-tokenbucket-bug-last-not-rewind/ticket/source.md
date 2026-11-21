# Token bucket persists a last in the past

THIS BUG ONLY. Lua HSET last will change (max(previous, t)); keep MIT notice. Do not also do idle-fill-to-burst, wait mapping, NaN, or ttl unless required.

Problem: consumeOne clamps elapsed when nowMicro < last but returns nowMicro as newLast — backward now is persisted. Lua HSET last, t the same. Memory samples m.now() BEFORE mu.Lock(), so a stale timestamp can overwrite a newer last. Extra refill / double grant.

Agreed how: Never persist a last in the past. Elapsed still uses the backward clamp so elapsed is not negative.
- consumeOne stores max(previous last, nowMicro), not raw nowMicro after the clamp.
- Lua HSET the same (last = max(bucket.last, t)).
- Memory reads now after mu.Lock() so lock order is the clock order.
Do not only fix the goroutine path. Do not drop the elapsed clamp.

Source: tokenbucket/BUGS.md item 6.

Tests first (hard): CREATE tests, confirm FAIL, then fix, then PASS. Copy/adapt: tokenbucket/repro_hunt_clock_last_backward_test.go — TestRepro_StaleNowRewindsLastDoubleRefill (sequential + goroutines_stale_samples_before_fresh_lock). Helpers newBurst1Delay0 / mustAllow are in that file; copy them or inline.
