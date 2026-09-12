# Test coverage

1. [hard] Edge case untested — `leakybucket/redis.go:79` — Redis `Add` special-cases `n < 1` with `errPour`; `TestMemory_AddRejectsNLessThanOne` hits only `Memory.Add`. No test calls `Redis.Add` with `n < 1`. Reverting the Redis guard leaves that suite green: exact `Add(0)` becomes `evalPour` (Level), and `Add(-1)` pours a negative delta.
   → Assert `Redis.Add(key, 0)` and `Add(key, -1)` return `errPour` and do not EVAL / change water
   Status: done
   Argument: `TestRedis_AddRejectsNLessThanOne`; `c8855c3`.
2. [hard] Edge case untested — `leakybucket/redis.go:286` — unparseable EVAL until field returns `errEvalUntil`; `TestRedis_EvalBadReply` asserts `errEvalLen`, `errEvalWater`, and `errEvalFlag` only (`(none)` for until)
   → Assert a 3-field EVAL reply with a non-numeric third bulk returns `errEvalUntil`
   Status: done
   Argument: `TestRedis_EvalBadReply` asserts `errEvalUntil`; `c8855c3`.
3. [judgement] Happy path only — `leakybucket/redis.go:99` — `Redis.Level` buffered arm (`levelBuffered`) is never called; Level is proven on memory (`TestMemory_LevelAfterDeltaT`) and exact Redis (`TestMemoryAndRedis_Agree`, two-instance `reader.Level` at `sync_rate=0`)
   → Assert buffered Level equals leaked Redis water plus `local_pours` and a following Take does not see an extra pour
   Status: skipped
   Argument: judgement; exact Level and two-instance buffered Take already prove the clock.
4. [judgement] Happy path only — `leakybucket/redis.go:164` — `Wake` after `Sleep` (not `Close`) should restart the flush ticker; `TestClose_StopsTickerAndKeepsRedis` only asserts `Wake` after `Close` does not start one
   → Assert `Sleep` then `Wake` (limiter not closed) starts the ticker again
   Status: skipped
   Argument: judgement; Close/Wake already covers the reclaim death path.
