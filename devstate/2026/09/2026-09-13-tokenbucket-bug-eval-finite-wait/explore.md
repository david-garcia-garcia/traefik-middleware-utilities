# Explore

## Concepts

```
  Redis.Allow
       │
       ▼
  Eval 3-field reply: true, wait, tokens
       │
       ▼
  strconv.ParseFloat(values[1])
       │
       ├─ err != nil ──────────► (false, 0, errEvalWait)   // "xyz"
       │
       └─ err == nil
            │
            ├ dest today: waitDuration + allowedFromWait → (allowed, wait, nil)
            │   nan / +Inf / inf → Duration overflow (MinInt64) → allowed true
            │   -Inf → waitDuration 0 → allowed true
            │
            └ desired: math.IsNaN / math.IsInf → (false, 0, errEvalWait)
```

`errEvalWait` already owns “wait is not a number”. Finite is the same job: a string that is not a usable wait. Do not add a second error.

`allowedFromWait` / bug 1’s `waitMicro <= maxDelayMicro` do not own this: `-Inf <= maxDelayMicro` is true, so `-Inf` still admits.

Identity is not reconstructed here. The caller already owns the opaque key (`openspec/specs/std_go_tokenbucket_allow/spec.md`).

## Decisions

- Fix only `Redis.Allow` after a successful `ParseFloat` of `values[1]`: if `math.IsNaN(waitMicro) || math.IsInf(waitMicro, 0)` return `(false, 0, errEvalWait)`. Same sentinel as `"xyz"` in `TestRedis_EvalBadReply`.
- Land `tokenbucket/repro_eval_nan_wait_test.go` `TestRepro_EvalWaitNaNFailOpen` first (fail on dest, then pass). Adapt dest `Allow(ctx, key) (bool, time.Duration, error)`. Reuse `startTestFakeRedis` / `setEvalReply` / `arrayBulks` / `newSimpleRedisForTest`.
- After the error, assert `errors.Is(err, errEvalWait)` and wait `0` — dest already returns `0` on `"xyz"`.
- Do not change `Memory.Allow`, Lua `allowScript`, `waitDuration`, or `allowedFromWait`. Do not use bug 1’s microsecond compare as this fix.
- Spec/usage catch-up: fold a finite-wait rule into `std_go_tokenbucket_allow` (Redis mapping, not a new deny). Usage Gotcha on `std_go_tokenbucket.md` after the code is true.
- Tests first: create the repro, run, confirm FAIL, then the one-line finite check, then PASS. Existing `go test -short ./tokenbucket` stays green.

## Open questions

- Q: Where do the new tests live?
  Rank: additive asked — new test this change creates; Desired names tests that fail on dest
  Decision: resolved — `tokenbucket/repro_eval_nan_wait_test.go` as the ticket named; dest three-return `Allow`.
  By: explore

- Q: Must the errEvalWait cases also assert wait is 0?
  Rank: additive incidental — Desired names non-nil `errEvalWait`; dest `"xyz"` already returns wait 0
  Decision: assumed — assert wait `0` and `errors.Is(err, errEvalWait)` like `TestRedis_EvalBadReply`. Do not give non-finite wait a Duration meaning.
  By: explore

- Q: Can live Lua `tostring(wait_duration)` emit `nan` / `inf`, or only a fake 3-field override?
  Rank: additive asked — Desired still requires the finite check even if Lua never emits it
  Decision: assumed — still require finite after ParseFloat. Proof is the fake override. Do not wait for a live Lua nan path. Do not change Lua in this ticket.
  By: explore
