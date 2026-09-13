Developer review: in progress — 2026-09-13T06:15:55Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Motivation
On dest, `Redis.Allow` maps a 3-field Eval wait with `strconv.ParseFloat`. A garbage string becomes `errEvalWait`. `nan`, `+Inf`, `-Inf`, and `inf` parse with `err == nil`, skip that sentinel, and go through `waitDuration` then `allowedFromWait`.

That path admits. Measured on dest: wait `"nan"` / `"+Inf"` / `"inf"` returned `allowed=true`, wait `MinInt64` Duration, `err=nil`. `"-Inf"` returned `allowed=true`, wait `0`, `err=nil`. `-Inf <= maxDelayMicro` is true, so bug 1’s microsecond compare would still admit `-Inf`.

If this stays unmerged, a 3-field Eval wait that is not a finite number fail-opens instead of `errEvalWait`. Callers treat that as a successful consume.

```mermaid
sequenceDiagram
  participant Allow
  participant ParseFloat
  participant waitDuration
  participant allowedFromWait
  Allow->>ParseFloat: wait nan / Inf
  ParseFloat-->>Allow: err nil
  Note over Allow: skip errEvalWait
  Allow->>waitDuration: non-finite
  waitDuration-->>Allow: 0 or overflow Duration
  Allow->>allowedFromWait: wait vs maxDelay
  allowedFromWait-->>Allow: true
  Allow-->>Allow: allowed true, err nil
```

## Merge readiness
Explore is written; product apply has not started.

Priority: P1 — serving a wrong public contract today
Reviewed head: ecb420a
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI still in progress; no product apply yet |
| CI proof | 3/6 | Lint succeeded; other checks queued on the stub PR |
| Local tests proof | N/A | Before implement (`localTests: none`) |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-tokenbucket-bug-eval-finite-wait pushed | `git` / pr-host |
| OpenSpec | none | no change folder |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/53 | pr-host List |
| CI | build 34742165109 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742165109 | pr-host check_runs (head `385b7e9`) |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | pr-host get_comments empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-tokenbucket-bug-eval-finite-wait` from `origin/master`. Stub PR #53 is the durable card. Explore reproduced dest fail-open. Next is propose.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Must the errEvalWait cases also assert wait is 0? | additive incidental | assumed — assert wait 0 and `errors.Is(err, errEvalWait)` like `TestRedis_EvalBadReply`. Do not give non-finite wait a Duration meaning. | explore |
| Can live Lua `tostring(wait_duration)` emit `nan` / `inf`, or only a fake 3-field override? | additive asked | assumed — still require finite after ParseFloat. Proof is the fake override. Do not wait for a live Lua nan path. Do not change Lua in this ticket. | explore |

## Before merge
- [ ] Land tokenbucket tests that fail on dest for Eval wait `nan` / `+Inf` / `-Inf` / `inf`, then require finite `ParseFloat` or `errEvalWait`
- [ ] Fold the finite-wait contract into the allow spec and usage packet

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | ecb420ac7846afccb4f0e329772b44cc9558b03f | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest still admits a non-finite Eval wait; the agreed fix is `errEvalWait` after `ParseFloat` when the number is not finite.

Do we have a high-confidence way to reproduce? Yes. Throwaway `TestThrowaway_EvalWaitNaNFailOpen` on dest (deleted, not committed): all four waits `nan` / `+Inf` / `-Inf` / `inf` returned `allowed=true`, `err=nil`. ParseFloat `err` was nil; `IsNaN`/`IsInf` true.

Is this the best way to solve the issue? Yes vs dest: one finite check, same sentinel as a garbage string, no second error type, no deny-with-nil.

### Evidence
What I checked:
- Throwaway dest test FAIL: `Allow` wait `nan`/`+Inf`/`inf` → `(true, MinInt64 Duration, nil)`; `-Inf` → `(true, 0, nil)` (`go test -run TestThrowaway_EvalWaitNaNFailOpen ./tokenbucket`, dest `d69f89d` code)
- `strconv.ParseFloat` of those four strings: `err=<nil>`
- `-Inf <= float64(time.Second.Microseconds())` is true
- `tokenbucket/redis.go` ParseFloat then only `convErr`
- `tokenbucket/clock.go` `waitDuration` / `allowedFromWait` / `errEvalWait`
- `tokenbucket/limiter_test.go` `TestRedis_EvalBadReply` (`"xyz"` only)
- CI run 34742165109 in progress (pr-host check_runs on `385b7e9`)

### Rank-up moves
None.
