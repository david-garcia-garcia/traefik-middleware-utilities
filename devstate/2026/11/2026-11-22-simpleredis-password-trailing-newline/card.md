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
Ready for review. 0 items remain.

Priority: unknown — motivation not written
Reviewed head: 45a0cb7b
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996727348 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-simpleredis-password-trailing-newline pushed | `git` |
| OpenSpec | simpleredis-password-trailing-newline | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/115 | pr-host |
| CI | build 44996727348 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996727348 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996727348 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-simpleredis-password-trailing-newline on branch 2026-11-22-simpleredis-password-trailing-newline targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/115; CI build 44996727348 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996727348. This change documents and tests SimpleRedis AUTH against upstream issue traefik/traefik#12078 (https://github.com/traefik/traefik/issues/12078): Config.Pass is sent byte for byte, with no trimming; a trailing newline from a Secret or file makes AUTH fail.

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
| Reviewed head | 45a0cb7b831a38c9f1c3d12f630a0e497c145a3b | Card must match the branch you measured |

### Stored data model
None.
