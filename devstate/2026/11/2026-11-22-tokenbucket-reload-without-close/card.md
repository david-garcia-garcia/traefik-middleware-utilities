## Summary
Solo test de regresión: 50 recargas sin Close no deben dejar sockets proxied ni goroutines. Sin cambio de producto. El defecto de Traefik no se reproduce aquí.

La existencia del defecto se tomó de estos informes upstream:
- Issue https://github.com/traefik/traefik/issues/13618
- PR upstream https://github.com/traefik/traefik/pull/13619 (abierto)

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
Reviewed head: 44446dab
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
| Branch | 2026-11-22-tokenbucket-reload-without-close pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/113 | pr-host |
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
Ticket 2026-11-22-tokenbucket-reload-without-close on branch 2026-11-22-tokenbucket-reload-without-close targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/113; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Se conserva el umbral `leakedGoroutines > 2` del repro o se ajusta en dest? | additive asked — Tests pide conservar casos de control; Unknowns deja el umbral a implement al transcribir | assumed — transcribir `leakedGoroutines > 2`. Hoy el repro midió +0. Implement solo lo aprieta con evidencia dest; no lo afloja. | explore |


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
| Reviewed head | 44446dabbe59cc7e48f868b678ecb21b2f6984d0 | Card must match the branch you measured |

### Stored data model
None.
