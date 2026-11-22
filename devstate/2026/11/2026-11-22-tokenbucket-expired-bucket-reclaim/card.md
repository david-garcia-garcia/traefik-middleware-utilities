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
Reviewed head: c55da0d4
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
Ninguna.

## How this fits together
Ticket 2026-11-22-tokenbucket-expired-bucket-reclaim en la rama 2026-11-22-tokenbucket-expired-bucket-reclaim hacia master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/111; origen del defecto: https://github.com/traefik/traefik/issues/13704; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Cómo adaptar los tests de referencia a `Allow(ctx, key) (Status, error)` sin copiar helpers de otros issues? | additive asked — ficheros de test que este cambio crea; la línea Tests del Design nombra ficheros autocontenidos con sufijo 13704 | assumed — llamar `Allow` como `(status, err)` y fallar si `err != nil`. El test Memory cuenta `len(limiter.buckets)`. El e2e declara `liveKey13704` y `evalKey13704` en su propio fichero. | explore |
| Propose añade un escenario al spec vivo para diez+uno / PTTL o deja el catálogo como está? | additive incidental — escenario nuevo en un leaf que ya existe; el Design dice No product change y no nombra un delta de spec | assumed — fold de un escenario en `std_go_tokenbucket_allow` para diez ociosas más una nueva = una fuente. No añadir un SHALL de `PTTL`; el e2e observa el TTL restante del `EXPIRE` ya contratado. | explore |
| El test Memory usa reloj de test o espera el ttl real? | additive asked — Tests nombra `TestTraefik13704_MemoryReclaimsExpiredBuckets`, que congela el reloj | assumed — `SetNowForTest` y salto `ttl+9s`. No dormir el ttl de pared. | explore |
| Hace falta un `traefik_issue13704_yaegi_test.go` además de los dos ficheros nombrados? | additive incidental — Tests nombra dos ficheros; no un fichero Yaegi | assumed — solo esos dos. El tope/caducidad interpretado ya lo cubre `TestYaegi_MemoryCapAndExpiry`. | explore |

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
| Reviewed head | c55da0d4004bd61fd144bc82a561052016eaf5bf | La tarjeta debe coincidir con la rama medida |

### Stored data model
Ninguno.
