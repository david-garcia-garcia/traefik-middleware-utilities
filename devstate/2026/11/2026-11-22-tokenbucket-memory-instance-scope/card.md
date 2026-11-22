## Motivation
Cuando varios routers de Traefik apuntan al mismo middleware RateLimit, el limitador en memoria y el de Redis no comparten el mismo ámbito. Traefik construye un limitador in-memory por router, de modo que N routers dan N presupuestos por origen; Redis clavea por middleware (`rate:<middleware>:<source>`) y comparte uno. Esta librería ya replica ese corte: cada `*Memory` es su propio mapa de cubos; `Redis.Allow` evalúa la `key` opaca del llamante en el servidor. El defecto no se redescubrió aquí: viene de Traefik [#13706](https://github.com/traefik/traefik/issues/13706), con el PR abierto de opción `scope` [#13823](https://github.com/traefik/traefik/pull/13823) y el PR cerrado [#13707](https://github.com/traefik/traefik/pull/13707).

El hueco era de contrato, no de código. El GoDoc de `Memory` / `NewMemory` no decía que cada valor sea un espacio de claves ni cómo compartir presupuesto entre routers o recargas. El de `Redis` / `NewRedis` no decía que el estado vive en el servidor, que cualquier limitador sobre la misma clave lo comparte, ni que el prefijo define el ámbito. El catálogo vivo `std_go_tokenbucket_allow` exigía que el prefijo de `key` sea del llamante, pero no nombraba Memory-por-instancia frente a Redis-por-clave. El packet de uso ya afirmaba el share de Redis y omitía el hecho simétrico: dos `*Memory` sobre la misma clave admiten `2*burst`. Tampoco había prueba que afirmara ese contrato: el repro de referencia esperaba un burst compartido entre dos `NewMemory` y medía 6 admisiones frente a burst 3.

Si se deja sin nombrar, un llamante trata el desacuerdo de Traefik como bug de esta librería, y un consumidor que llama `NewMemory` una vez por router obtiene N presupuestos en memoria y uno en Redis para el mismo límite configurado. Cambiar de almacén entonces aprieta el límite sin tocar la config. Traefik midió límites decenas o cientos de veces más holgados que lo configurado cuando muchos routers comparten un middleware. El arreglo a nivel middleware (un `*Memory` por nombre, al estilo #13823) no cabe en este paquete.

Priority: P3 — spec, documentación, tests o claridad interna, sin daño actual para usuarios u operadores.

## Implementation
Sin cambio de `Allow` ni de constructores. El ámbito existente pasa a ser promesa escrita y comprobada en `tokenbucket`.

GoDoc de `Memory` / `NewMemory`: cada valor es su propio espacio de claves; `Allow` en uno no consume tokens de otro; para compartir presupuesto entre routers o recargas hay que reutilizar un `*Memory` (ejemplo: tabla `reclaim` claveada por nombre de middleware, sin importar `reclaim` desde este paquete). GoDoc de `Redis` / `NewRedis`: el estado vive en el servidor; cualquier `*Redis` sobre la misma clave comparte el presupuesto; el llamante prefija la clave para elegir el ámbito.

Los tests autocontenidos, con helpers sufijo `13706` y la API dest `(Status, error)`, afirman ese contrato: dos llamantes que comparten un `*Memory` admiten `burst` en total; dos `*Memory` distintos admiten `2*burst` (comentario que cita `traefik/traefik#13706`); en vivo, dos `*Redis` sobre una clave admiten `burst` en total, con el mismo gate que `TestLive_Limiter`. El packet de uso gana el gotcha simétrico (dos `*Memory` = `2*burst`) junto al share de Redis, y el catálogo `std_go_tokenbucket_allow` registra las mismas dos promesas. El arreglo de middleware queda como nota de deuda: el consumidor debe compartir un `*Memory` si quiere que memoria y Redis coincidan.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** El contrato de `Allow` ahora nombra el ámbito: cada `*Memory` es su propio espacio de claves (dos valores sobre la misma `key` admiten `2*burst`; hay que compartir un valor para un presupuesto, p. ej. `reclaim` por nombre de middleware); cualquier `*Redis` sobre la misma `key` comparte el estado del servidor (admiten `burst` en total); el prefijo sigue siendo del llamante. El cableado a nivel middleware no está en esta librería.
**End users.** None.

## Merge readiness
In progress. 1 items remain.

Priority: P3 — spec, documentación, tests o claridad interna, sin daño actual para usuarios u operadores.
Reviewed head: 62cdd67d
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
| Branch | 2026-11-22-tokenbucket-memory-instance-scope pushed | `git` |
| OpenSpec | tokenbucket-memory-instance-scope | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/109 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/openspec/changes/archive/2026-11-22-tokenbucket-memory-instance-scope/proposal.md) — modified

Completed:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/openspec/specs/std_go_tokenbucket_allow/spec.md) — modified


## Deviations from the ask
None.

## Follow-up issues
- [ ] [Consumer should share one Memory limiter per middleware name](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/knowledge/debt/2026-11-22-tokenbucket-consumer-limiter-scope.md) — el arreglo de ámbito (un `*Memory` compartido por nombre de middleware, mismo prefijo Redis) pertenece al middleware consumidor, no a esta librería. Origen: https://github.com/traefik/traefik/issues/13706 ; PRs https://github.com/traefik/traefik/pull/13823 y https://github.com/traefik/traefik/pull/13707.


## How this fits together
Ticket 2026-11-22-tokenbucket-memory-instance-scope on branch 2026-11-22-tokenbucket-memory-instance-scope targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/109; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿El catálogo vivo necesita un requisito de ámbito (Memory por instancia, Redis por clave) o basta GoDoc + tests? | additive asked — Desired «El ámbito pasa a ser contrato documentado»; binding «Make the scope part of the documented contract»; Affected nombra `openspec/specs/std_go_tokenbucket_allow/spec.md`; el delta añade una promesa, los callers existentes siguen valiendo | assumed — ADDED en `std_go_tokenbucket_allow`: cada `*Memory` es su propio espacio de claves; cualquier `*Redis` sobre la misma `key` comparte el estado del servidor; el prefijo sigue siendo del llamante. GoDoc es la superficie encargada; el catálogo registra la misma promesa. No nueva familia. | explore |


## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/devstate/2026/11/2026-11-22-tokenbucket-memory-instance-scope/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/devstate/2026/11/2026-11-22-tokenbucket-memory-instance-scope/codereview_nitpicks.md) — 2 total, 0 pending, 2 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/devstate/2026/11/2026-11-22-tokenbucket-memory-instance-scope/codereview_spec.md) — 2 total, 0 pending, 0 completed, 2 skipped
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/devstate/2026/11/2026-11-22-tokenbucket-memory-instance-scope/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/devstate/2026/11/2026-11-22-tokenbucket-memory-instance-scope/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/devstate/2026/11/2026-11-22-tokenbucket-memory-instance-scope/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/devstate/2026/11/2026-11-22-tokenbucket-memory-instance-scope/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/devstate/2026/11/2026-11-22-tokenbucket-memory-instance-scope/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 62cdd67dc572a0227ec38dd15cab5e852cb7f1ae | Card must match the branch you measured |

### Stored data model
None.
