Developer review: in progress — 2026-09-13T06:17:32Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `tokenbucket-reject-nonfinite-rate` folds construction fail for NaN and Inf into `std_go_tokenbucket_allow`. Product apply not landed.

**End users.** None.

## Motivation
On dest, `NewMemory` and `NewRedis` are the only gates before a token-bucket `Allow`. Passthrough is skipping construction; this package has no unlimited-rate constructor.

`validateClock` only rejects `rate <= 0`. `NaN <= 0` is false and `+Inf > 0` is true, so both constructors accept those rates. A NaN rate makes every `Allow` true. A +Inf rate does `Inf*0 = NaN` on the second `Allow` at elapsed=0, so `maxDelay=0` still admits.

If this stays unmerged, a caller that passes a non-finite rate gets a limiter that fail-opens instead of an `errRate`. The agreed fix is reject NaN and Inf at `New` as `errRate`; do not special-case `Allow`.

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
Propose artifacts are apply-ready; product apply has not started.

Priority: P1 — serving a wrong public contract today
Reviewed head: f1f97cf
Owner decision: None.

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
| Branch | 2026-09-13-tokenbucket-bug-finite-rate pushed | `git push` `f1f97cf` |
| OpenSpec | tokenbucket-reject-nonfinite-rate | `openspec/changes/tokenbucket-reject-nonfinite-rate/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/52 | pr-host List |
| CI | build 34742382649 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742382649 | pr-host CI |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-finite-rate/openspec/changes/tokenbucket-reject-nonfinite-rate/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-tokenbucket-bug-finite-rate` from `origin/master`. Stub PR #52 is the durable card. Propose is apply-ready; implement next.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Do new tests land as the named `repro_*.go` files or fold into `limiter_test.go`? | additive asked | assumed — land `repro_nan_rate_test.go` and `repro_hunt_inf_rate_nan_test.go`; adapt dest Allow signature | explore |
| Does `-Inf` get its own constructor test, given dest already rejects it via `rate <= 0`? | additive incidental | assumed — include `-Inf` beside +Inf; dest-green `-Inf` is not the fail-first proof | explore |
| Does `TestRepro_NaNRateAllowFailOpen` stay as a skip-after-fix witness or become New-reject only? | additive asked | assumed — keep the test; Skip when New rejects NaN | explore |
| Should `errRate` text change from "greater than 0" to mention finite? | additive incidental | assumed — keep the existing string; callers match `errors.Is` | explore |

## Before merge
- [ ] Land tokenbucket tests that fail on dest for NaN and Inf at `New`, then reject non-finite rate in `validateClock` as `errRate`
- [ ] Fold non-finite rate into the tokenbucket allow spec and usage packet

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
| Reviewed head | f1f97cfefda71fc0c6d67af66f1bc0b0cdd29da1 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Dest still accepts NaN and Inf at `New`; the ticket’s `validateClock` finite-rate gate is the how.

Do we have a high-confidence way to reproduce? Yes — throwaway `go test` on dest: NaN and +Inf constructors succeed and Allow fail-opens; `-Inf` already `errRate`.

Is this the best way to solve the issue? Yes — special-casing `Allow` or `consumeOne` would paper over a constructor that should have failed.

### Evidence
What I checked:
- OpenSpec change `tokenbucket-reject-nonfinite-rate` validates strict
- FindSpecHost fold `std_go_tokenbucket_allow` (high)
- CI run 34742382649 queued after propose push

### Rank-up moves
None.
