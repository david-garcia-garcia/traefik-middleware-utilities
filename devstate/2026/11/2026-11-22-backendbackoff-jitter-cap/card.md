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
Reviewed head: 7aa9a159
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
| Branch | 2026-11-22-backendbackoff-jitter-cap pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/124 | pr-host |
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
Ticket 2026-11-22-backendbackoff-jitter-cap on branch 2026-11-22-backendbackoff-jitter-cap targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/124; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Which jitter-at-cap formula meets Desired (never exceed Max, no point mass at the cap, Jitter 0 still disables)? | bounded asked — `cooldownDuration` already has 2 production callers (`backendbackoff/allow.go:72`, `:86`; roots: worktree, pattern `cooldownDuration`, 4 hits / 2 files) and this change would migrate them here; Desired names "no single value carries a point mass at the cap; jittered probes stay spread when the nominal cooldown is at the cap". | assumed — leave the choice to tech-lead propose among the listed options | explore |


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
| Reviewed head | 7aa9a1590c07b9ca8a95d9f60f8aff02613d4f93 | Card must match the branch you measured |

### Stored data model
None.
