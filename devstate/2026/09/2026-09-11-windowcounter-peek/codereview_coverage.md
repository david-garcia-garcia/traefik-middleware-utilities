# Test coverage

1. [hard] Critical path untested — `windowcounter/limiter.go:192` — buffered Peek must not increment `local_delta`; `TestPeek_DoesNotIncrement` / live `peekThenTake` / Yaegi `PeekThenTake` all `New(client, 0)`; `TestPeek_BufferedSkipStormDoesNotGetEveryCall` only asserts GET count
   → Assert N buffered Peeks then Take sees estimate 1
   Status: done
   Argument: TestPeek_BufferedDoesNotIncrement — N buffered Peeks then Take estimate 1 (`74bbc68`).
2. [hard] Critical path untested — `windowcounter/limiter.go:217` — buffered Peek estimate is `redis_known + local_delta` so Peek after Takes can deny; no test Takes then Peeks with `sync_rate > 0` (`TestPeek_AgreesWithTakeBeforeIncrement` and `TestPeek_StaysDeniedThenSlidesAllowed` are exact; skip-storm discards allowed/estimated)
   → Assert buffered Takes then Peek matches Take-before-increment (denied once the local estimate exceeds the limit)
   Status: done
   Argument: TestPeek_BufferedTakeThenPeekDenies — Takes until over limit then Peek denied (`74bbc68`).
