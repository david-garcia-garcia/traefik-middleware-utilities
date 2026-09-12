# Standards

1. [hard] Leave a trail — `simpleredis/bench_test.go:118` — `encodeGet`, `encodeEval`, `decodeBulk`, `decodeArray10`, `decodeInteger`, `cannedBulkGET`, `decodeBulk100KB`, and `encodeSet100KB` are new methods with no succinct job comment
   → Add a one-line job comment on each helper (encode/decode loop or canned `$n` payload), not a restatement of the identifier
   Status: done
   Argument: f0460da job comments on encode/decode helpers.
2. [hard] Leave a trail — `simpleredis/bench_test.go:105` — `repeatReader.Read` has no job comment
   → Comment that it copies the canned RESP payload into `p`, wrapping at the end
   Status: done
   Argument: f0460da Read comment.
3. [hard] Leave a trail — `simpleredis/bench_test.go:377` — `allocExceedsCeiling` and `assertAllocCeiling` have no job comment
   → Comment that the first reports over-budget allocs/op or B/op, and the second fails the Test when either ceiling is exceeded
   Status: done
   Argument: f0460da allocExceedsCeiling and assertAllocCeiling comments.
4. [hard] Leave a trail — `simpleredis/bench_test.go:32` — `BenchmarkGet`, `BenchmarkMGet10`, `BenchmarkIncr`, and `BenchmarkEval` have no job comment
   → Add a one-line comment that each is the end-to-end fake-server measurement for that verb
   Status: done
   Argument: f0460da BenchmarkGet/MGet10/Incr/Eval comments.
5. [judgement] Duplicated Code — `simpleredis/bench_test.go:264` — `startSlowRedis` copies `startFakeRedis` listen/accept/count scaffolding and only changes the per-command sleep plus canned `$1` reply
   → Parameterize the existing fake server with latency, or leave the copy; churn tests need overlapping in-flight, not the RESP map
   Status: skipped
   Argument: judgement; outside the encode/decode job; churn needs overlapping in-flight, not the RESP map.
