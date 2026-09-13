Developer review: in progress — 2026-09-13T06:10:14Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

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
Prepare is grounded; product apply has not started. Explore is next.

Priority: P1 — serving a wrong public contract today
Reviewed head: 8ce4193
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
| Branch | 2026-09-13-tokenbucket-bug-finite-rate pushed | `git push` `8ce4193` |
| OpenSpec | none | no change folder |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/52 | pr-host Create |
| CI | build 34742075608 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742075608 | pr-host CI |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-tokenbucket-bug-finite-rate` from `origin/master` (tokenbucket present; `origin/HEAD` is stale `initial`). Stub PR #52 is the durable card. Next is explore.

## Explore Decisions
None.

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
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 8ce4193bc4e002269fcd6783db7906a9ff9719e6 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Dest still accepts NaN and Inf at `New`; the ticket’s `validateClock` finite-rate gate is the how.

Do we have a high-confidence way to reproduce? Yes, `NewMemory`/`NewRedis` with `math.NaN()` and `math.Inf(1)` on dest `validateClock`.

Is this the best way to solve the issue? Yes — special-casing `Allow` or `consumeOne` for NaN tokens would paper over a constructor that should have failed, and +Inf is not an unlimited-rate constructor.

### Evidence
What I checked:
- `tokenbucket/` exists on `origin/master` at `d69f89d` (`git ls-tree`)
- `validateClock` rate `<= 0` only (`tokenbucket/clock.go`); `NewMemory` / `NewRedis` call it
- Dest tests reject rate 0 only (`tokenbucket/limiter_test.go` `TestNewMemory_RejectsInvalidClock`)
- Stub PR #52, comment inventory empty, CI run 34742075608 queued

### Rank-up moves
None.
