## Motivation
Not yet.

## Implementation
Not yet.

## Upstream

La existencia del defecto se toma de estos informes upstream, no de un hallazgo local independiente:

- Issue: https://github.com/traefik/traefik/issues/14173
- Issue: https://github.com/traefik/traefik/issues/13879
- PR upstream (abierto): https://github.com/traefik/traefik/pull/13880
- PR upstream (cerrado como duplicado): https://github.com/traefik/traefik/pull/14236

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Merge readiness
Ready for review. 0 items remain.

Priority: unknown — motivation not written
Reviewed head: e6db7ca8
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996362034 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-tokenbucket-canceled-request pushed | `git` |
| OpenSpec | tokenbucket-canceled-request | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/112 | pr-host |
| CI | build 44996362034 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996362034 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996362034 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-tokenbucket-canceled-request on branch 2026-11-22-tokenbucket-canceled-request targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/112; CI build 44996362034 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996362034. Defect taken from https://github.com/traefik/traefik/issues/14173, https://github.com/traefik/traefik/issues/13879, https://github.com/traefik/traefik/pull/13880, and https://github.com/traefik/traefik/pull/14236.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Dragonfly 1.40.2 sigue sin ejecutar el EVAL retenido en este árbol? | additive asked — no reshape de callers; Unknowns nombra la medición; Desired pide registrar si se cobró, no un assert por motor | assumed — el test live registra el bucket tras el hold y no falla por cobro vs vacío. Apply en este árbol: Redis cobró (`tokens=2` de 3); Dragonfly cobró en una corrida (`tokens=2`) y quedó vacío en otra. Medición 2026-11-22 (análisis): Redis cobró; Dragonfly vacío. | implement |


## Findings
None.

## Axis review
None.

## Review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | e6db7ca8d5d6f7b9b8fec3c2dd26ceaf3edaacc7 | Card must match the branch you measured |

### Stored data model
None.
