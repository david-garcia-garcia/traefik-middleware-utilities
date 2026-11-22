## Motivation
Un bucket inactivo que caduca se crea otra vez lleno. Con 30 peticiones por minuto y burst 30, el TTL que calcula Traefik es 3 s, pero el bucket tarda 60 s en llenarse. Medido en Memory: se vacía el burst, pasan 5 s en el reloj, y entran otras 30 peticiones en lugar de 2. Redis y Dragonfly hacen lo mismo porque un hash caducado se siembra con tokens iguales a burst.

El defecto se toma de https://github.com/traefik/traefik/issues/13957. No hay pull request upstream.

Si se deja así, un cliente que espera un poco más que ese TTL recupera el burst entero.

Priority: P1 — el límite admite un burst nuevo tras una pausa más corta que el rellenado

## Implementation
Tras validar el ttl del llamador, `NewMemory` y `NewRedis` guardan en el reloj compartido el máximo entre ese ttl y el techo de rellenado: los segundos enteros de burst/rate más maxDelay, sin pasar del mayor `time.Duration` en segundos enteros. Memory y Redis caducan con ese único valor. La firma no cambia y un ttl fraccionario sigue rechazándose.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** El argumento `ttl` de `NewMemory` y `NewRedis` es un mínimo: la librería lo sube al tiempo de rellenado y los dos almacenes caducan con ese valor.
**End users.** Tras una pausa corta ya no reciben un burst nuevo.

## Merge readiness
Ready for review. 0 items remain.

Priority: P1 — el límite admite un burst nuevo tras una pausa más corta que el rellenado
Reviewed head: c6eb7fcd
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44995343447 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-tokenbucket-ttl-below-refill pushed | `git` |
| OpenSpec | tokenbucket-ttl-refill-floor | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/108 | pr-host |
| CI | build 44995343447 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44995343447 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44995343447 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/openspec/changes/tokenbucket-ttl-refill-floor/proposal.md) — modified
- [std_go_tokenbucket_lua-eval](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/openspec/changes/tokenbucket-ttl-refill-floor/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-tokenbucket-ttl-below-refill on branch 2026-11-22-tokenbucket-ttl-below-refill targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/108; CI build 44995343447 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44995343447.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/devstate/2026/11/2026-11-22-tokenbucket-ttl-below-refill/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/devstate/2026/11/2026-11-22-tokenbucket-ttl-below-refill/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/devstate/2026/11/2026-11-22-tokenbucket-ttl-below-refill/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/devstate/2026/11/2026-11-22-tokenbucket-ttl-below-refill/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/devstate/2026/11/2026-11-22-tokenbucket-ttl-below-refill/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/devstate/2026/11/2026-11-22-tokenbucket-ttl-below-refill/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/devstate/2026/11/2026-11-22-tokenbucket-ttl-below-refill/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-ttl-below-refill/devstate/2026/11/2026-11-22-tokenbucket-ttl-below-refill/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 walked / 0 open | Unanswered review is merge risk |
| Reviewed head | c6eb7fcdcdcc23155e56d1e68e04421bd228e490 | Card must match the branch you measured |

### Stored data model
None.
