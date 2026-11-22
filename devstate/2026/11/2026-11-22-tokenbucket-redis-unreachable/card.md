## Motivation
El middleware de rate-limit llama a `Redis.Allow` para consumir un token vía Eval. El `rateLimit` de Traefik responde HTTP 500 cuando Redis está caído y no ofrece fail-open ni fail-closed. Este ticket fija ese corte en la ruta de consume de esta biblioteca para que el caller elija la política en vez de heredar ese 500.

Cuando Redis no responde, Traefik responde HTTP 500 `Could not insert/update bucket` sin opción fail-open ni fail-closed (https://github.com/traefik/traefik/issues/12091). Esta biblioteca ya devuelve el error de Eval con un `Status` cero (`Allowed` false) y no elige código HTTP. Nada dedicado prueba que un loopback que rechaza es `errors.Is(err, simpleredis.ErrUnreachable)` con `allowed=false`. El GoDoc de `Allow` no nombra `ErrUnreachable` ni `ErrTimeout`, ni que fail-open o fail-closed lo decide el middleware. El usage todavía pedía comparar errores Redis por el texto de `Error()`, y el contrato de allow describía el corte como la cadena `redis:unreachable`.

Si no se fija, un cambio posterior puede tragar el corte como un deny silencioso, los callers siguen comparando cadenas de display que pueden derivar, y los autores de middleware no pueden ver en esta biblioteca que ellos eligen el código HTTP.

Priority: P3 — spec, docs, tests o claridad interna — sin daño actual a usuario u operador

## Implementation
Sin cambio de comportamiento: `Redis.Allow` sigue devolviendo `Status{}, err` si Eval falla. El GoDoc de ese método nombra `simpleredis.ErrUnreachable` (o `ErrTimeout`) con `allowed=false` en un corte de Redis y que fail-open o fail-closed lo decide el middleware, citando el issue de Traefik. Un test unitario autónomo apunta el limiter a `127.0.0.1:1` con `MaxRetries: -1` y afirma `errors.Is(err, simpleredis.ErrUnreachable)` y `allowed=false`. El How to use compara esos sentinels con `errors.Is` / `IsUnreachable`; el texto de display sigue `redis:unreachable`. El delta del contrato de allow nombra el mismo matching y mantiene el corte como error, no como deny sustituto. Los tests de `Error()` existentes siguen como control del texto de display.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** Comparar cortes Redis con `errors.Is` / `IsUnreachable` contra `ErrUnreachable` y `ErrTimeout`; `Allow` documenta eso con `allowed=false`, y fail-open o fail-closed lo decide el middleware.
**End users.** None.

## Merge readiness
Listo para revisión. 0 ítems pendientes.

Priority: P3 — spec, docs, tests o claridad interna — sin daño actual a usuario u operador
Reviewed head: 7977d39a
Owner decision: Requerido. Ver Explore Decisions.

## Review scores
| Medida | Resultado | Qué significa |
| --- | --- | --- |
| Preparación global | 6/6 | Listo |
| Prueba de CI | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44997670008 |
| Prueba de tests locales | N/A | PR remoto — la prueba de CI cubre esto |
| Resolución de review | 6/6 | sin comentarios de PR abiertos |

## Verification
| Comprobación | Resultado | Evidencia |
| --- | --- | --- |
| Rama | 2026-11-22-tokenbucket-redis-unreachable pushed | `git` |
| OpenSpec | tokenbucket-redis-unreachable | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/114 | pr-host |
| CI | build 44997670008 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44997670008 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44997670008 |
| Tests locales | passed | handoff.yaml localTests |
| Comentarios de PR | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/openspec/changes/tokenbucket-redis-unreachable/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-tokenbucket-redis-unreachable en la rama 2026-11-22-tokenbucket-redis-unreachable hacia master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/114; CI build 44997670008 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44997670008.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Basta el GoDoc de `Redis.Allow` o hay que alinear `knowledge/devdocs/std_go_tokenbucket.md:28` («Match Redis errors by `Error()` text») con `errors.Is`? | additive incidental — una línea de usage existente; ningún criterio del Design nombra el usage (Affected lo deja a explore/propose; el pedido nombra GoDoc). | assumed — alinear How to use a `errors.Is` / `IsUnreachable` contra `ErrUnreachable` y `ErrTimeout`, igual que `std_go_simpleredis.md`. El snippet que solo propaga `err != nil` puede quedarse como un fail-closed de ejemplo; el GoDoc dice que la política es del middleware. No tocar `std_go_windowcounter.md`. | explore |


## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/devstate/2026/11/2026-11-22-tokenbucket-redis-unreachable/codereview_standards.md) — 1 total, 0 pending, 1 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/devstate/2026/11/2026-11-22-tokenbucket-redis-unreachable/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/devstate/2026/11/2026-11-22-tokenbucket-redis-unreachable/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/devstate/2026/11/2026-11-22-tokenbucket-redis-unreachable/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/devstate/2026/11/2026-11-22-tokenbucket-redis-unreachable/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/devstate/2026/11/2026-11-22-tokenbucket-redis-unreachable/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/devstate/2026/11/2026-11-22-tokenbucket-redis-unreachable/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/devstate/2026/11/2026-11-22-tokenbucket-redis-unreachable/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Métrica | Valor | Por qué importa |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Misma lista que ## Specs |
| Comentarios de review abiertos recorridos | 0 FIX / 0 ANSWER / 0 open | Un review sin responder es riesgo de merge |
| Reviewed head | 7977d39a487b6765f981a0295dae4c7e2e29e094 | La tarjeta debe coincidir con la rama medida |

### Stored data model
None.
