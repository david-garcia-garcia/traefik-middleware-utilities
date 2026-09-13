Developer review: in progress — 2026-09-13T06:19:27Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `tokenbucket-eval-finite-wait` folds a finite Eval-wait rule into `std_go_tokenbucket_allow`. Product apply has not landed.

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
Propose is written; product apply has not started.

Priority: P1 — serving a wrong public contract today
Reviewed head: d4dfdc7
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI still queued; no product apply yet |
| CI proof | 3/6 | Checks queued on the stub PR |
| Local tests proof | N/A | Before implement (`localTests: none`) |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-tokenbucket-bug-eval-finite-wait pushed | `git` / pr-host |
| OpenSpec | tokenbucket-eval-finite-wait | `openspec/changes/tokenbucket-eval-finite-wait/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/53 | pr-host List |
| CI | build 34742359935 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742359935 | pr-host check_runs |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | pr-host get_comments empty |

## Specs
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-eval-finite-wait/openspec/changes/tokenbucket-eval-finite-wait/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-tokenbucket-bug-eval-finite-wait`, stub PR #53. Propose folded finite wait into `std_go_tokenbucket_allow`. Next is implement (tests first).

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
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | d4dfdc7108de9f8b513ae3aa79ee0f8d4f2886bd | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest still admits a non-finite Eval wait; the agreed fix is `errEvalWait` after `ParseFloat` when the number is not finite.

Do we have a high-confidence way to reproduce? Yes. Dest throwaway test failed on all four waits (explore). Implement will land `TestRepro_EvalWaitNaNFailOpen` first.

Is this the best way to solve the issue? Yes vs dest: one finite check, same sentinel as a garbage string, no second error type, no deny-with-nil.

### Evidence
What I checked:
- `openspec/changes/tokenbucket-eval-finite-wait/` artifacts complete (`openspec validate`)
- FindSpecHost fold `std_go_tokenbucket_allow` (candidates include `std_go_tokenbucket_lua-eval`)
- CI run 34742359935 queued (pr-host check_runs)

### Rank-up moves
None.
