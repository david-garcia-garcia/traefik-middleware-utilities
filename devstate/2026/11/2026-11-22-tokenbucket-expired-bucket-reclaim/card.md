## Summary
Tests de regresión: Memory libera los buckets caducados cuando llega una clave nueva; las claves Redis llevan EXPIRE. El defecto se tomó del issue upstream https://github.com/traefik/traefik/issues/13704 (no hay PR upstream). Este producto no reproduce el fallo; el cambio es solo tests.

## Motivation
Aún no.

## Implementation
Aún no.

## What this changes
**Operators.** Ninguno.

**Admin users.** Ninguno.

**Developers.** Ninguno.

**End users.** Ninguno.

## Merge readiness
En curso. 0 elementos pendientes.

Priority: unknown — motivación aún no escrita
Reviewed head: 33976bd3
Owner decision: Ninguna.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | No listo |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | PR remoto — la prueba de CI cubre esto |
| Review resolution | 6/6 | no hay comentarios abiertos en el PR |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-tokenbucket-expired-bucket-reclaim pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/111 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Ninguna.

## Deviations from the ask
Ninguna.

## Follow-up issues
Ninguna. El ticket pedía diferir `dropExpired`/`dropOne`; dest ya tiene `makeRoomLocked` y los bancos.

## How this fits together
Ticket 2026-11-22-tokenbucket-expired-bucket-reclaim en la rama 2026-11-22-tokenbucket-expired-bucket-reclaim hacia master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/111; origen del defecto: https://github.com/traefik/traefik/issues/13704; CI not seen.

## Explore Decisions
Ninguna.

## Findings
Ninguna.

## Axis review
Ninguna.

## Review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Misma lista que ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Un comentario sin responder es riesgo de merge |
| Reviewed head | 33976bd3d310eabb382efb72cd6cb2cf37fabe6c | La tarjeta debe coincidir con la rama medida |

### Stored data model
Ninguno.
