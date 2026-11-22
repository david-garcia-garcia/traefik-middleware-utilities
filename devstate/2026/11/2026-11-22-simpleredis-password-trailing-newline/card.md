The existence of this defect was taken from https://github.com/traefik/traefik/issues/12078 (cause identified in comments: Kubernetes Secret values ending in a newline).

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
Reviewed head: 626f7cc1
Owner decision: None.

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
| Branch | 2026-11-22-simpleredis-password-trailing-newline pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/115 | pr-host |
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
Ticket 2026-11-22-simpleredis-password-trailing-newline on branch 2026-11-22-simpleredis-password-trailing-newline targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/115; CI not seen. Upstream: https://github.com/traefik/traefik/issues/12078 (no upstream PR).

## Explore Decisions
None.

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
| Reviewed head | 626f7cc175f70da7878d94e29e3d5d6651c3a0ae | Card must match the branch you measured |

### Stored data model
None.
