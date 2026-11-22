## Motivation
El middleware de rate limit llama a `Allow` y traduce el error a HTTP. Traefik responde 500 cuando el cliente ya se ha ido porque su `select` no distingue una cancelación de un error del servidor. La existencia de ese defecto se toma de los informes upstream https://github.com/traefik/traefik/issues/14173, https://github.com/traefik/traefik/issues/13879, https://github.com/traefik/traefik/pull/13880 y https://github.com/traefik/traefik/pull/14236, no de un hallazgo local independiente.

En esta librería el 500 no ocurre: `Allow` ya devuelve `context.Canceled` con `Allowed` falso y el paquete no escribe HTTP. Lo que falta es el contrato escrito y la prueba de dos caminos. En `Memory`, un `ctx` ya terminado no cobra, de modo que el siguiente `Allow` sigue teniendo el burst completo; eso no estaba dicho ni cubierto más allá de un `Status` a cero. En `Redis`, un retorno `context.Canceled` o `DeadlineExceeded` no prueba que el script no haya corrido: si la petición se cancela cuando el `EVALSHA` ya está en el cable, `Allow` vuelve pronto con `Canceled` pero el servidor puede ejecutar el script y cobrar un token.

Si se deja así, quien mapee `Canceled` a 499 puede perder un token de un cliente que ya se fue, y un cambio futuro en `Memory` podría empezar a cobrar una petición cancelada y recortar el burst del siguiente `Allow`. El 500 de Traefik sigue siendo el mapeo del middleware, no de este paquete.

Priority: P3 — documentación y tests, sin daño actual a usuarios u operadores

## Implementation
Sin cambio de comportamiento de `Allow`. Se amplía el GoDoc de `Memory.Allow` (un `ctx` ya terminado devuelve su error y no cobra) y de `Redis.Allow` (`Canceled` / `DeadlineExceeded` no prueba que el script no haya corrido; el token puede haberse cobrado). Ambos comentarios sitúan el mapeo HTTP (499 si `Canceled`, no esperar si `wait==0`) en el middleware y citan traefik/traefik#14173.

`TestTraefik14173_MemoryCanceledContext` afirma `Canceled`, `Allowed` falso, `Status` cero y burst completo después. `TestTraefik14173_RedisCanceledInFlight` cancela `Allow` mientras un proxy TCP retiene el `EVALSHA` (hold 1500 ms, cancel a los 200 ms), afirma retorno en menos de la mitad del hold y registra si se cobró el token, sin un bucle Lua ocupado con `TIME`. Helpers propios con sufijo `14173`; misma puerta live que `TestLive_Limiter`.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** El GoDoc de `Memory.Allow` y `Redis.Allow` fija el contrato de cancelación: `Memory` no cobra si el `ctx` ya terminó; en `Redis`, `Canceled` o `DeadlineExceeded` no prueba que el script no haya corrido. El mapeo HTTP 499 pertenece al middleware (traefik/traefik#14173).
**End users.** None.

## Merge readiness
Ready for review. 0 items remain.

Priority: P3 — documentación y tests, sin daño actual a usuarios u operadores
Reviewed head: 1805e384
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
Ticket 2026-11-22-tokenbucket-canceled-request on branch 2026-11-22-tokenbucket-canceled-request targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/112; CI build 44996362034 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996362034.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Dragonfly 1.40.2 sigue sin ejecutar el EVAL retenido en este árbol? | additive asked — no reshape de callers; Unknowns nombra la medición; Desired pide registrar si se cobró, no un assert por motor | assumed — el test live registra el bucket tras el hold y no falla por cobro vs vacío. Apply en este árbol: Redis cobró (`tokens=2` de 3); Dragonfly cobró en una corrida (`tokens=2`) y quedó vacío en otra. Medición 2026-11-22 (análisis): Redis cobró; Dragonfly vacío. | implement |


## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-canceled-request/devstate/2026/11/2026-11-22-tokenbucket-canceled-request/codereview_standards.md) — 2 total, 0 pending, 2 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-canceled-request/devstate/2026/11/2026-11-22-tokenbucket-canceled-request/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-canceled-request/devstate/2026/11/2026-11-22-tokenbucket-canceled-request/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-canceled-request/devstate/2026/11/2026-11-22-tokenbucket-canceled-request/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-canceled-request/devstate/2026/11/2026-11-22-tokenbucket-canceled-request/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-canceled-request/devstate/2026/11/2026-11-22-tokenbucket-canceled-request/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-canceled-request/devstate/2026/11/2026-11-22-tokenbucket-canceled-request/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-canceled-request/devstate/2026/11/2026-11-22-tokenbucket-canceled-request/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 1805e384ef242555528607959ef75d944bd84910 | Card must match the branch you measured |

### Stored data model
None.
