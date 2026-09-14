Developer review: ready for review — 2026-09-13T06:47:46Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `validateClock` rejects NaN and Inf as `errRate`. Unit tests `TestRepro_NaNRateRejected` and `TestRepro_InfRateRejected` prove dest used to accept those rates; `TestRepro_NaNRateAllowFailOpen` skips after New rejects. Live spec `std_go_tokenbucket_allow` construction fail now names non-finite rate.

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
Apply, archive, and CI succeeded. Ready for review.

Priority: P1 — serving a wrong public contract today
Reviewed head: 6956f05
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | CI succeeded; no open review comments |
| CI proof | 6/6 | All 8 checks succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742922116 |
| Local tests proof | N/A | prHost remote; CI proof covers remote |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-tokenbucket-bug-finite-rate pushed | `git push` `6956f05` |
| OpenSpec | tokenbucket-reject-nonfinite-rate | `openspec/changes/archive/2026-09-13-tokenbucket-reject-nonfinite-rate/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/52 | pr-host List |
| CI | build 34742922116 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742922116 | pr-host CI |
| Local tests | passed | `go test -short ./...` |
| PR comments | no comments | inventory empty |

## Specs
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/openspec/changes/archive/2026-09-13-tokenbucket-reject-nonfinite-rate/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-tokenbucket-bug-finite-rate` from `origin/master`. PR #52 is the durable card. CI run 34742922116 succeeded.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Do new tests land as the named `repro_*.go` files or fold into `limiter_test.go`? | additive asked | assumed — land `repro_nan_rate_test.go`; Inf tests as `repro_inf_rate_test.go` after review renamed the hunt filename | codereview |
| Does `-Inf` get its own constructor test, given dest already rejects it via `rate <= 0`? | additive incidental | assumed — include `-Inf` beside +Inf; dest-green `-Inf` is not the fail-first proof | explore |
| Does `TestRepro_NaNRateAllowFailOpen` stay as a skip-after-fix witness or become New-reject only? | additive asked | assumed — keep the test; Skip when New rejects NaN | explore |
| Should `errRate` text change from "greater than 0" to mention finite? | additive incidental | assumed — keep the existing string; callers match `errors.Is` | explore |

## Before merge
None.

## Findings
- [Standards 1](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/devstate/2026/09/2026-09-13-tokenbucket-bug-finite-rate/codereview_standards.md) — FIX — Inf test file renamed to `repro_inf_rate_test.go`. Path: `tokenbucket/repro_inf_rate_test.go`. Reply none.
- [Nitpicks 1](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/devstate/2026/09/2026-09-13-tokenbucket-bug-finite-rate/codereview_nitpicks.md) — FIX — Inf test renamed to `TestRepro_InfRateRejected`. Path: `tokenbucket/repro_inf_rate_test.go`. Reply none.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/devstate/2026/09/2026-09-13-tokenbucket-bug-finite-rate/codereview_standards.md) — 2 total, 0 pending, 1 completed, 1 skipped
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/devstate/2026/09/2026-09-13-tokenbucket-bug-finite-rate/codereview_nitpicks.md) — 1 total, 0 pending, 1 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/devstate/2026/09/2026-09-13-tokenbucket-bug-finite-rate/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/devstate/2026/09/2026-09-13-tokenbucket-bug-finite-rate/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/devstate/2026/09/2026-09-13-tokenbucket-bug-finite-rate/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/devstate/2026/09/2026-09-13-tokenbucket-bug-finite-rate/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/devstate/2026/09/2026-09-13-tokenbucket-bug-finite-rate/codereview_coverage.md) — 0 total, 0 pending, 0 completed

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 6956f0593900e162ee8be998ec1b8da4c816e3da | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: `validateClock` rejects NaN and Inf as `errRate`; Inf tests named for constructor reject; live catalog names non-finite construction fail.

Do we have a high-confidence way to reproduce? Yes — fail-then-pass: dest tests failed for NaN and +Inf New and NaN Allow fail-open; after the gate the same command passed; `go test -short ./...` passed; CI run 34742922116 succeeded.

Is this the best way to solve the issue? Yes — the shared constructor gate is the owner. `errRate` string left as dest (public Error() text).

### Evidence
What I checked:
- FAIL `go test -short -count=1 -timeout 60s -run 'TestRepro_NaNRate|TestRepro_InfRate' ./tokenbucket` before the gate
- PASS same command after `rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0)`
- `go test -short ./...` passed
- CI Lint, Unit, Unit race, Go E2E Redis, Go E2E Dragonfly, Integration Tests, Integration Tests Redis, Integration Tests Dragonfly all success on run 34742922116

### Rank-up moves
None.
