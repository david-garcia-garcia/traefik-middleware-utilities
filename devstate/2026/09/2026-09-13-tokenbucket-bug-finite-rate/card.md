Developer review: in progress — 2026-09-13T06:20:57Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `validateClock` rejects NaN and Inf as `errRate`. Unit tests in `repro_nan_rate_test.go` and `repro_hunt_inf_rate_nan_test.go` prove dest used to accept those rates and Allow fail-opened.

**End users.** None.

## Motivation
On dest, `NewMemory` and `NewRedis` are the only gates before a token-bucket `Allow`. Passthrough is skipping construction; this package has no unlimited-rate constructor.

`validateClock` only rejected `rate <= 0`. `NaN <= 0` is false and `+Inf > 0` is true, so both constructors accepted those rates. A NaN rate made every `Allow` true. A +Inf rate did `Inf*0 = NaN` on the second `Allow` at elapsed=0, so `maxDelay=0` still admitted.

If this stays unmerged, a caller that passes a non-finite rate gets a limiter that fail-opens instead of an `errRate`.

```mermaid
sequenceDiagram
  participant Caller
  participant NewMemory
  participant validateClock
  participant Allow
  Caller->>NewMemory: rate NaN or +Inf
  NewMemory->>validateClock: rate <= 0 only
  validateClock-->>NewMemory: nil
  NewMemory-->>Caller: limiter
  Caller->>Allow: first consume
  Allow-->>Caller: true
  Caller->>Allow: second at elapsed 0
  Note over Allow: Inf*0 = NaN or NaN tokens
  Allow-->>Caller: true, maxDelay 0
```

## Merge readiness
Apply landed locally (`go test -short ./...` passed). Remote CI still queued.

Priority: P1 — serving a wrong public contract today
Reviewed head: 6986195
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI still queued |
| CI proof | 3/6 | Checks queued after the fix push |
| Local tests proof | N/A | prHost remote; CI proof covers remote |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-tokenbucket-bug-finite-rate pushed | `git push` `6986195` |
| OpenSpec | tokenbucket-reject-nonfinite-rate | `openspec/changes/tokenbucket-reject-nonfinite-rate/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/52 | pr-host List |
| CI | build 34742502193 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742502193 | pr-host CI |
| Local tests | passed | `go test -short ./...` |
| PR comments | no comments | inventory empty |

## Specs
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/openspec/changes/tokenbucket-reject-nonfinite-rate/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-tokenbucket-bug-finite-rate` from `origin/master`. Stub PR #52 is the durable card. Apply landed; code review next.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Do new tests land as the named `repro_*.go` files or fold into `limiter_test.go`? | additive asked | assumed — land `repro_nan_rate_test.go` and `repro_hunt_inf_rate_nan_test.go`; adapt dest Allow signature | explore |
| Does `-Inf` get its own constructor test, given dest already rejects it via `rate <= 0`? | additive incidental | assumed — include `-Inf` beside +Inf; dest-green `-Inf` is not the fail-first proof | explore |
| Does `TestRepro_NaNRateAllowFailOpen` stay as a skip-after-fix witness or become New-reject only? | additive asked | assumed — keep the test; Skip when New rejects NaN | explore |
| Should `errRate` text change from "greater than 0" to mention finite? | additive incidental | assumed — keep the existing string; callers match `errors.Is` | explore |

## Before merge
- [x] Land tokenbucket tests that fail on dest for NaN and Inf at `New`, then reject non-finite rate in `validateClock` as `errRate`
- [ ] Archive the allow spec so the live catalog names non-finite rate
- [ ] Wait for CI on PR #52

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 6986195c22bc4663abc762cebeba425f3965115a | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: `validateClock` now rejects NaN and Inf as `errRate`; Allow and consumeOne were not special-cased.

Do we have a high-confidence way to reproduce? Yes — fail-then-pass: dest tests failed (`NewMemory(NaN)`, `NewRedis(NaN)`, `NewMemory(+Inf)`, `NewRedis(+Inf)`, NaN Allow fail-open); after `validateClock` the same command passed; `go test -short ./...` passed.

Is this the best way to solve the issue? Yes — the shared constructor gate is the owner.

### Evidence
What I checked:
- FAIL `go test -short -count=1 -timeout 60s -run 'TestRepro_NaNRate|TestRepro_InfRate' ./tokenbucket` before the gate (NaN New, +Inf New, NaN Allow always true). `-Inf` already `errRate`.
- PASS same command after `tokenbucket/clock.go` `rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0)`
- `go test -short ./...` passed (backendbackoff, reclaim, simpleredis, tokenbucket, windowcounter)
- CI run 34742502193 queued on `6986195`

### Rank-up moves
None.
