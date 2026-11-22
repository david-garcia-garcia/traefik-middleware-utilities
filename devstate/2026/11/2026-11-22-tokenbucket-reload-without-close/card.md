## Motivation
Traefik reconstruye un cliente Redis en cada recarga de middleware y no lo cierra. El limitador Redis de este árbol usa simpleredis, no go-redis: New no arranca trabajo de fondo, Close drena los sockets ociosos, y un cliente silencioso que sigue alcanzable puede conservar ociosos hasta Close. La fuga upstream es https://github.com/traefik/traefik/issues/13618 (PR abierto https://github.com/traefik/traefik/pull/13619): desde go-redis v9.12 cada UniversalClient huérfano fija un CircuitBreakerManager.cleanupLoop, así que goroutines y memoria crecen con cada recarga.

Esa fuga es de Traefik. Aquí no ocurre: simpleredis no arranca ese bucle, y los clientes abandonados sueltan los sockets cuando dejan de ser alcanzables. Falta la prueba de ese contrato con el mismo patrón de recarga: cincuenta veces un cliente y un limitador Redis nuevos, un Allow, y se descartan sin Close. El camino de ciclos de Close del catálogo vivo solo cubre New, uso y Close, no el descarte sin Close.

Sin esa prueba, un New posterior que arranque una goroutine de fondo, o un pool que conserve sockets proxied después de que el cliente pase a basura, enviaría la misma clase de fuga por recarga que Traefik, sin candado live en Redis ni Dragonfly.

Priority: P3 — tests and catalog proof with no current user or operator harm

## Implementation
Una regresión live en el paquete tokenbucket simula esas cincuenta recargas a través de un proxy TCP de loopback que solo cuenta sockets abiertos (accept más copia bidireccional; sin retención de escrituras). Cada recarga construye simpleredis.New en la dirección del proxy, NewRedis, un Allow con la firma existente (Status, error), y suelta ambos sin Close. Tras asentar el GC, el recuento de sockets abiertos del proxy debe ser 0 y las goroutines filtradas (después menos antes menos tres por socket abierto restante, excluyendo las del propio proxy) fallan solo cuando superan 2. El test reutiliza el gate live existente para que Redis y Dragonfly lo ejecuten, y cierra el cliente de espera antes del bucle de recarga para que ese socket no entre en el conjunto descartado. Un requisito aditivo en std_go_simpleredis_tcp-session registra que los clientes abandonados sin Close no dejan sockets tras el GC y que New sigue sin arrancar una goroutine de fondo, sin cambiar idle-hasta-Close para un cliente que sigue alcanzable.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** Un cliente SimpleRedis inalcanzable descartado sin Close no debe dejar sockets TCP tras el GC, y New sigue sin arrancar una goroutine de fondo.
**End users.** None.

## Merge readiness
In progress. 0 items remain.

Priority: P3 — tests and catalog proof with no current user or operator harm
Reviewed head: 06584948
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
| OpenSpec | tokenbucket-reload-without-close | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/113 | pr-host |
| CI | not seen | ci-host |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/openspec/changes/archive/2026-11-22-tokenbucket-reload-without-close/proposal.md) — modified

Completed:
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/openspec/specs/std_go_simpleredis_tcp-session/spec.md) — modified


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
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/devstate/2026/11/2026-11-22-tokenbucket-reload-without-close/codereview_standards.md) — 2 total, 0 pending, 2 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/devstate/2026/11/2026-11-22-tokenbucket-reload-without-close/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/devstate/2026/11/2026-11-22-tokenbucket-reload-without-close/codereview_spec.md) — 2 total, 0 pending, 2 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/devstate/2026/11/2026-11-22-tokenbucket-reload-without-close/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/devstate/2026/11/2026-11-22-tokenbucket-reload-without-close/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/devstate/2026/11/2026-11-22-tokenbucket-reload-without-close/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/devstate/2026/11/2026-11-22-tokenbucket-reload-without-close/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/devstate/2026/11/2026-11-22-tokenbucket-reload-without-close/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 06584948224d75e32b6b454257051ab102977608 | Card must match the branch you measured |

### Stored data model
None.
