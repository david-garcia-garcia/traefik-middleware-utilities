## Summary
Tests de regresión: Memory libera los buckets caducados cuando llega una clave nueva; las claves Redis llevan EXPIRE. El defecto se tomó del issue upstream https://github.com/traefik/traefik/issues/13704 (no hay PR upstream). Este producto no reproduce el fallo; el cambio es solo tests. OpenSpec: `tokenbucket-expired-bucket-reclaim` (fold de un escenario en `std_go_tokenbucket_allow`).

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
Reviewed head: be15bcc2
Owner decision: Obligatorio. Ver Explore Decisions.

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
| OpenSpec | tokenbucket-expired-bucket-reclaim | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/111 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/openspec/changes/tokenbucket-expired-bucket-reclaim/specs/std_go_tokenbucket_allow/spec.md) — modified
- `std_go_tokenbucket_lua-eval` — skip (sin contrato PTTL; EXPIRE entero ya está; el e2e observa el TTL restante)

## Deviations from the ask
Ninguna.

## Follow-up issues
Ninguno.

## How this fits together
Ticket 2026-11-22-tokenbucket-expired-bucket-reclaim en la rama 2026-11-22-tokenbucket-expired-bucket-reclaim hacia master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/111; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Cómo adaptar los tests de referencia a Allow(ctx, key) (Status, error) sin copiar helpers de otros issues? | additive asked — ficheros de test que este cambio crea; la línea Tests del Design nombra ficheros autocontenidos con sufijo 13704 | assumed — llamar Allow como (status, err) y fallar si err != nil. El test Memory no usa countAdmits; cuenta len(limiter.buckets) bajo el mutex. El e2e declara liveKey13704 y evalKey13704 en traefik_issue13704_e2e_test.go (Eval pttl con ScriptSHA1Hex, KEYS[1] para Dragonfly). No copiar liveKey/evalKey del árbol de repros ni de otros issues. | propose |
| Propose añade un escenario al spec vivo para diez+uno / PTTL o deja el catálogo como está? | additive incidental — escenario nuevo en un leaf que ya existe; el Design dice "No product change" y no nombra un delta de spec | assumed — fold de un escenario en el requisito Memory store caps sources and removes expired ones first de std_go_tokenbucket_allow: diez fuentes ociosas más allá de ttl más una nueva dejan exactamente una fuente almacenada. No añadir un SHALL de PTTL en std_go_tokenbucket_lua-eval; el EXPIRE entero ya es el contrato; el e2e observa el TTL restante. | propose |
| El test Memory usa reloj de test o espera el ttl real, como podría leerse el brief? | additive asked — Tests nombra TestTraefik13704_MemoryReclaimsExpiredBuckets, que congela el reloj | assumed — SetNowForTest y salto ttl+9s, igual que la referencia y que memory_test.go. No dormir el ttl de pared. | propose |
| Hace falta un traefik_issue13704_yaegi_test.go además de los dos ficheros nombrados? | additive incidental — Tests nombra dos ficheros; no un fichero Yaegi | assumed — solo esos dos. El tope/caducidad interpretado ya lo cubre TestYaegi_MemoryCapAndExpiry. | propose |

## Findings
Ninguno.

## Axis review
Ninguno.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | be15bcc22d1a3890f3e1e290fbf71f82d403d3c7 | Card must match the branch you measured |

### Stored data model
Ninguno.
