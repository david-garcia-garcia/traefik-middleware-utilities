# Test coverage

1. [hard] Edge case untested — `windowcounter/limiter.go:281` — Take keeps memory when previous `localDelta > 0`; spec scenario Unflushed previous delta is not GET; test `(none)` (repro Sleeps first so `localDelta` is 0; `TestTake_BufferedPendingDeltaOutage` never rolls onto previous with pending delta)
   → Assert a buffered Take after a window roll with unflushed previous does not GET that key and uses `redis_known + local_delta` from memory
   Status: done
   Argument: `TestTake_BufferedUnflushedPreviousNotGet` asserts estimated 2 from unflushed previous and one extra GET (new current only). SHA 6a4f015.
