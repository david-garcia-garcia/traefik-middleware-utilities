# Requirement
IssueKey: 2026-09-13-tokenbucket-bug-eval-finite-wait

## Problem
`Redis.Allow` treats a 3-field Eval wait of `nan` / `+Inf` / `-Inf` / `inf` as a successful parse, skips `errEvalWait`, and admits (`allowed=true`, `err=nil`). Fail-open. This bug only.

## Current (code)
- `tokenbucket/redis.go` `Allow` — `strconv.ParseFloat(string(values[1]), 64)` then `convErr != nil` → `(false, 0, errEvalWait)`. No `math.IsNaN` / `math.IsInf` after a successful parse. Then `waitDuration` + `allowedFromWait` + `nil`.
- `tokenbucket/clock.go` `errEvalWait` — `"tokenbucket: eval wait is not a number"`. Same sentinel for garbage strings only.
- `tokenbucket/clock.go` `waitDuration` — `waitMicro <= 0` returns `0` (`-Inf` takes this path). Else `time.Duration(waitMicro * float64(time.Microsecond))`.
- `tokenbucket/clock.go` `allowedFromWait` — `wait <= maxDelay`. Zero wait and a Duration overflow to a large negative both compare `<= maxDelay` as true after New (maxDelay is not negative).
- `tokenbucket/limiter_test.go` `TestRedis_EvalBadReply` — len≠3 → `errEvalLen`; wait `"xyz"` → `errEvalWait`. No `nan` / Inf cases.
- Dest `tokenbucket/` has no `repro_eval_nan_wait_test.go` and no `BUGS.md`.
- `openspec/specs/std_go_tokenbucket_allow/spec.md` — Redis errors MUST NOT become deny / fail-open / fail-close inside Allow; scenarios name `redis:unreachable` / `redis:timeout` only. No finite-wait rule.
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` — Go MUST NOT GET/SET the hash; wait mapping is not specified as finite.
- `knowledge/devdocs/std_go_tokenbucket.md` — Lua always returns `"true"`; Go maps allowed from wait vs maxDelay. No non-finite wait.

## Desired
1. Land product tests in `tokenbucket/` that **fail on current dest** (`go test ./tokenbucket`). Do not weaken them to pass. Tests first, then the fix.
2. Reproduce a fake Redis 3-field reply (`true`, wait, `0`) for waits `nan`, `+Inf`, `-Inf`, `inf`. Reuse `startTestFakeRedis` / `setEvalReply` / `arrayBulks` / `newSimpleRedisForTest`. Adapt the caller example `TestRepro_EvalWaitNaNFailOpen` to dest `Allow(ctx, key) (bool, time.Duration, error)`.
3. Those tests fail while Allow returns `allowed=true` and `err=nil` (or any `err=nil`). After the fix they require a non-nil error that is `errEvalWait`.
4. Agreed how: after a successful `ParseFloat`, require a finite number (`math.IsNaN` / `math.IsInf`). Otherwise return `errEvalWait` — same sentinel as `"xyz"`. Do not admit. Do not return `allowed=false` with `err=nil`. Do not add a second error type.
5. Bug 1’s microsecond compare does not replace this: `-Inf <= maxDelayMicro` is true, so `-Inf` would still admit without this check. NaN / `+Inf` on that compare would deny with `err=nil`, which this ticket forbids.
6. Then those tests PASS and existing `tokenbucket` tests stay green.

## Affected
- `tokenbucket/redis.go` (`Allow` after `ParseFloat` of `values[1]`)
- `tokenbucket/` tests (new failing-then-green coverage; likely next to `TestRedis_EvalBadReply` or a new test file)
- `openspec/specs/std_go_tokenbucket_allow/spec.md` (Eval wait that is not a finite number is `errEvalWait`, not admit/deny)
- `knowledge/devdocs/std_go_tokenbucket.md` (non-finite Eval wait)

## Out of scope
- Other tokenbucket bugs (maxDelay Duration vs micro compare; non-finite `rate` at New; clock `last` rewind; cap eviction; ttl truncation)
- Returning `allowed=false` with `err=nil` for non-finite wait
- A second error type besides `errEvalWait`
- Using bug 1’s microsecond admit/refund compare as this fix
- Changing `Memory.Allow` or Lua `allowScript` for this ticket
- Copying the caller example’s two-return `Allow("k")` signature onto dest

## Unknowns
- Whether new tests land as `tokenbucket/repro_eval_nan_wait_test.go` or inside `limiter_test.go`.
- Whether implement also asserts wait is `0` on `errEvalWait` (dest already returns `0` on `"xyz"`).
- Whether live Lua `tostring(wait_duration)` can emit `nan`/`inf`, or only a fake 3-field override (ticket still wants the finite check).

## Tensions
- Ticket example `Allow("k")` returns `(allowed, err)`. Dest `Redis.Allow` is `Allow(ctx, key) (bool, time.Duration, error)` (`tokenbucket/redis.go`). Adapt; do not restore the two-return API.
- Ticket / BUGS.md snippets show `(false, errEvalWait)` / `(allowed, nil)`. Dest already returns a wait duration in the middle.
- Spec `std_go_tokenbucket_allow` “Redis errors do not become deny” names unreachable/timeout, not a non-finite wait field. Ticket wins; spec/usage must catch up in propose.
- Bug 1 (maxDelay units) is a different ticket. Do not treat its microsecond compare as this change.
