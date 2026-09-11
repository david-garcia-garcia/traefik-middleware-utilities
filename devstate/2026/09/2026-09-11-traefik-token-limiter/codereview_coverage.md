# Test coverage

1. [hard] Edge case untested — `tokenbucket/memory.go:58` — lazy ttl expire (`entry == nil || !now.Before(entry.expireAt)`) resets the bucket; tests freeze `now` so only the nil-key arm runs (`TestMemory_BurstAfterIdle`)
   → Assert Allow after ttl elapses (tiny rate, idle > ttl) admits a fresh burst, not a partial refill from the stale last/tokens
   Status: done
   Argument: TestMemory_IdlePastTTLStartsFull.
2. [hard] Edge case untested — `tokenbucket/redis.go:19` — `NewRedis` returns `errRedis` when the client is nil; `(none)`
   → Assert `NewRedis(nil, …)` returns `errRedis` and no limiter
   Status: done
   Argument: TestNewRedis_RejectsNil.
3. [hard] Critical path untested — `tokenbucket/redis.go:61` — Eval reply length ≠ 3 or wait unparseable returns `errEvalLen`; tests only cover a 3-field hit (`TestRedis_EvalEncoding`, `TestMemoryAndRedis_Agree`)
   → Assert Allow returns `errEvalLen` (not allowed false) when Eval is not 3 fields or wait is not a float
   Status: done
   Argument: TestRedis_EvalBadReply (len≠3 → errEvalLen; non-numeric wait → errEvalWait).
4. [judgement] Happy path only — `tokenbucket/clock.go:56` — `nowMicro < last` clamps last so elapsed is not negative; untested
   → Assert a backward test clock does not invent negative elapsed, or skip if only monotonic now is reachable
   Status: skipped
   Argument: judgement; monotonic test clock is the production now path.
