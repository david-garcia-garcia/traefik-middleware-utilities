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
Reviewed head: ecb0a8d
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
| Branch | 2026-10-16-backendbackoff-allow-nocontext pushed | `git` |
| OpenSpec | backendbackoff-allow-nocontext | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/101 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_backendbackoff_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/openspec/changes/backendbackoff-allow-nocontext/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-10-16-backendbackoff-allow-nocontext on branch 2026-10-16-backendbackoff-allow-nocontext targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/101; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Existen plugins Traefik u otros consumidores fuera de este repositorio que sigan en `Allow(ctx, key)` y deban migrarse en este PR? | additive asked — no reshape en árbol; **Unknowns** en `requirement.md` nombra la incertidumbre sin pedir migración externa | assumed — fuera de alcance de este repo; el cambio de firma es breaking a propósito; no bloquear implement; quien despliegue copias externas migra en su propio ciclo. | explore |


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
| Reviewed head | ecb0a8d71880711b96f00e4f9cc415e8e89b4fc4 | Card must match the branch you measured |

### Stored data model
None.
