## Motivation
Not yet.

## Implementation
Not yet.

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Merge readiness
In progress. 0 items remain.

Priority: unknown — motivation not written
Reviewed head: 2e301c76
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Not ready |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-backendbackoff-stale-probe-report pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/121 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-backendbackoff-stale-probe-report on branch 2026-11-22-backendbackoff-stale-probe-report targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/121; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Qué forma de identidad une Allow con Report (generación, token opaco, otro)? | bounded incidental — cambia el contrato Allow/Report que ya tiene 32 llamadas a Allow y 18 a Report (raíces: worktree `*.go` `gate.Allow(`/`gate.Report(`, identificador `backendbackoff`, `e2e/`); ningún criterio nombra generación vs token (Unknowns; Out of scope «Elegir el diseño») | assumed — deferred to propose | explore |


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
| Reviewed head | 2e301c767486718754a09767662b43f7911b091d | Card must match the branch you measured |

### Stored data model
None.
