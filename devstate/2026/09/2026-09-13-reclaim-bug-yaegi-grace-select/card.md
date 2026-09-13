Developer review: in progress — 2026-09-13T16:34:49Z

[sgsi-dev-ticket-status:2026-09-13-reclaim-bug-yaegi-grace-select]

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None. Prepare only: ticket grounded, stub PR opened against master. Product code is unchanged.

**End users.** None.

## Motivation
`reclaim` parks a grace waiter in an interpreted two-case `select` on a timer and a wake channel — the same Yaegi v0.16.1 shape that hung `windowcounter` on Go 1.21.13. The Traefik plugin probe arms `DefaultGrace` on every drop. If the timer wake is missed, `expire` never runs, so Close never runs. This phase does not yet change that path.

## Merge readiness
Prepare complete. Explore next: reproduce on Go 1.21.13 before any product edit. 1 item remains.

Priority: P1 — production Traefik plugin path can strand a grace waiter and never Close the stored value
Reviewed head: 88426b2
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Stub PR only; no apply yet |
| CI proof | 1/6 | Pushed; CI not seen |
| Local tests proof | N/A | Before implement |
| Review resolution | 6/6 | No PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-bug-yaegi-grace-select pushed | origin |
| OpenSpec | none | openspec/ |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/80 | pr-host Create |
| CI | not seen | not measured this Set |
| Local tests | none | handoff.yaml |
| PR comments | no comments | none |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-13-reclaim-bug-yaegi-grace-select` off `origin/master` → stub PR #80. Explore must hang on Go 1.21.13 before implement.

## Explore Decisions
None.

## Before merge
- [ ] Reproduce the Yaegi grace-select hang on Go 1.21.13 with a goroutine dump, then fix and prove CI green.

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | No apply yet |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | No comments |
| Reviewed head | 88426b268c3177e6d724844a6d4ddf1d97f7b927 | Card matches start commit |

### Stored data model
None.

### Technical review
Best possible solution: not chosen this phase.

Do we have a high-confidence way to reproduce? Not yet — explore will force `GOTOOLCHAIN=go1.21.13`.

Is this the best way to solve the issue? Not yet.

### Evidence
What I checked:
- `go version` with `GOTOOLCHAIN=go1.21.13` is `go1.21.13 windows/amd64`
- `waitGraceOrWake` is still `go` + `select` on `origin/master` `reclaim/table.go`
- OPEN PRs that also edit `reclaim/table.go`: #78, #77
- GitHub identity: David / deivid.garcia.garcia@gmail.com

### Rank-up moves
None.
