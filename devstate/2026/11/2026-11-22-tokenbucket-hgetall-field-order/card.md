## Motivation
El limitador Redis del token bucket lee el hash con `HGETALL` y asigna `last` al segundo elemento y `tokens` al cuarto. Redis y Valkey no garantizan ese orden: con codificación hashtable depende de la semilla de hash del proceso. La existencia del defecto se toma de https://github.com/traefik/traefik/issues/14297 y del arreglo upstream https://github.com/traefik/traefik/pull/14300.

Si `tokens` llega primero, `last` recibe el número de tokens y `tokens` recibe el reloj en microsegundos. El tiempo transcurrido se vuelve enorme, el bucket se rellena hasta `burst` en cada llamada y se queda en `burst - 1`. Con burst 5 se admitieron 10 de 10 peticiones. El limitador deja de limitar y no lo registra.

Dejarlo así abre el límite en cualquier proceso cuya semilla ponga `tokens` primero, y también cuando el hash se insertó en ese orden.

Priority: P1 — un proceso Redis con `tokens` antes que `last` admite todas las peticiones

## Implementation
El script pide `last` y `tokens` con `HMGET` por nombre. El bucket ya existe solo si los dos valores son truthy; si falta uno, siembra `tokens = burst` y `last` al reloj de la llamada, como hasta ahora. El resto del script (relleno, tope de espera, `HSET`, `EXPIRE`) no cambia. En Redis y en Dragonfly un campo ausente llega como booleano `false`, así que la siembra sigue corriendo.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** El script Redis lee `last` y `tokens` por nombre. Un bucket guardado cuenta como presente solo cuando los dos campos son truthy.
**End users.** Un bucket ya lleno vuelve a denegar al pasar de `burst`, también cuando el hash guarda `tokens` antes que `last`.

## Merge readiness
Ready for review. 1 items remain.

Priority: P1 — un proceso Redis con `tokens` antes que `last` admite todas las peticiones
Reviewed head: d31dce8a
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44995589684 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-tokenbucket-hgetall-field-order pushed | `git` |
| OpenSpec | tokenbucket-hmget-named-fields | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/107 | pr-host |
| CI | build 44995589684 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44995589684 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44995589684 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_lua-eval](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/openspec/changes/archive/2026-11-22-tokenbucket-hmget-named-fields/proposal.md) — modified

Completed:
- [std_go_tokenbucket_lua-eval](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/openspec/specs/std_go_tokenbucket_lua-eval/spec.md) — modified


## Deviations from the ask
None.

## Follow-up issues
- [ ] [Alinear el script de bench de simpleredis con HMGET por nombre](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/knowledge/debt/2026-11-22-simpleredis-bench-tokenbucket-hgetall.md) — `simpleredis/bench_test.go` conserva una copia de `tokenBucketScript` que lee `HGETALL` por posición.


## How this fits together
Ticket 2026-11-22-tokenbucket-hgetall-field-order on branch 2026-11-22-tokenbucket-hgetall-field-order targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/107; CI build 44995589684 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44995589684.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-hgetall-field-order/devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | d31dce8aa8fb29c5ca2278aa30e9ebdfb6ee5704 | Card must match the branch you measured |

### Stored data model
None.
