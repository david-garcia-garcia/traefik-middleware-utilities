# Redis.Allow fail-opens on non-finite Eval wait

THIS BUG ONLY. Do not fix other tokenbucket bugs.

## Problem
Redis.Allow uses strconv.ParseFloat on Eval wait. nan / +Inf / -Inf / inf parse with err==nil, skip errEvalWait, and Allow returns (true, nil) (fail-open). tokenbucket/redis.go around ParseFloat(values[1]).

## Agreed how
After a successful ParseFloat, require a finite number (math.IsNaN / math.IsInf). Otherwise return errEvalWait — same sentinel as a garbage string. Do not admit. Do not return allowed=false with err=nil. Do not add a second error type.
Bug 1’s microsecond compare does not replace this: -Inf <= maxDelayMicro is true.

Source: d:\repositories\traefik-middleware-utilities\tokenbucket\BUGS.md item 2.

## Tests first (hard)
CREATE tests that reproduce, RUN them, confirm FAIL on dest, then fix, then confirm PASS.
Copy/adapt: d:\repositories\traefik-middleware-utilities\tokenbucket\repro_eval_nan_wait_test.go — TestRepro_EvalWaitNaNFailOpen (fake Redis 3-field reply, waits nan, +Inf, -Inf, inf). Uses startTestFakeRedis / setEvalReply / arrayBulks / newSimpleRedisForTest already in the package tests.
