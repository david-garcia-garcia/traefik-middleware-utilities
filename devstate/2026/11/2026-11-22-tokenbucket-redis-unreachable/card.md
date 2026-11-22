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
En curso. 0 ítems pendientes.

Priority: unknown — motivation not written
Reviewed head: c7d3dc93
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | No listo |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | PR remoto — la prueba de CI cubre esto |
| Review resolution | 6/6 | sin comentarios abiertos |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-tokenbucket-redis-unreachable pushed | `git` |
| OpenSpec | tokenbucket-redis-unreachable | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/114 | pr-host |
| Upstream | https://github.com/traefik/traefik/issues/12091 | Traefik 500 sin fail-open/fail-closed |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | comments.md ausente |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-redis-unreachable/openspec/changes/tokenbucket-redis-unreachable/proposal.md) — modified

## Deviations from the ask
Ninguna.

## Follow-up issues
Ninguna.

## How this fits together
Propuesta OpenSpec `tokenbucket-redis-unreachable` en la rama 2026-11-22-tokenbucket-redis-unreachable hacia master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/114. TEST+DOC, sin cambio de comportamiento: el cuerpo de `Redis.Allow` sigue devolviendo el error de Eval; el delta pliega `errors.Is` / `ErrUnreachable` / `ErrTimeout` y `Allowed=false` en el Status cero. Aguas arriba Traefik responde 500 y no ofrece fail-open ni fail-closed: https://github.com/traefik/traefik/issues/12091. CI no visto.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Basta el GoDoc de `Redis.Allow` o hay que alinear `knowledge/devdocs/std_go_tokenbucket.md:28` («Match Redis errors by `Error()` text») con `errors.Is`? | additive incidental — una línea de usage existente; ningún criterio del Design nombra el usage (Affected lo deja a explore/propose; el pedido nombra GoDoc). | assumed — alinear How to use a `errors.Is` / `IsUnreachable` contra `ErrUnreachable` y `ErrTimeout`, igual que `std_go_simpleredis.md`. El snippet que solo propaga `err != nil` puede quedarse como un fail-closed de ejemplo; el GoDoc dice que la política es del middleware. No tocar `std_go_windowcounter.md`. | explore |
| ¿El spec vivo necesita un escenario que nombre `ErrUnreachable` / `ErrTimeout` (hoy el escenario de error dice «Redis error» y Unreachable casa el texto `redis:unreachable`)? | additive incidental — escenario extra o aclaración de un contrato ya vivo; ningún acceptance criterion nombra el spec (Affected: solo si propose añade el escenario). | assumed — propose pliega en `std_go_tokenbucket_allow` un escenario que nombra `errors.Is(err, simpleredis.ErrUnreachable)` (y `ErrTimeout` en GoDoc/SHALL existente) con Status cero / `Allowed` false. Conservar «does not return allowed false as a substitute»: el error es el outage, no un deny silencioso. `Error()` sigue siendo `redis:unreachable`. | explore |
| ¿Hace falta un test del camino `ErrTimeout` (el Design lo nombra en GoDoc; el test nombrado es solo puerto que rechaza)? | additive incidental — test extra no pedido por la sección Tests; Design nombra `ErrTimeout` solo en GoDoc. | assumed — no. GoDoc nombra `ErrTimeout`; el fichero 12091 cubre refuse → `ErrUnreachable`. Un peer colgado sería maquinaria nueva. `simpleredis` ya prueba la clasificación timeout. | explore |
| ¿Este issue necesita un test live contra Redis/Dragonfly? | additive incidental — suite e2e extra; el repro de referencia es unitario contra `127.0.0.1:1`; Tests dice «Si hay tests live» (condicional). | assumed — no. Unreachable no necesita motor. Si más tarde hubiera live en ese fichero, el gating sería `runForEachLiveEngine` / `waitLiveClient` como `TestLive_Limiter`. El camino feliz live ya existe. | explore |
| ¿Cómo adaptar la firma del repro de referencia (`Allow` devolvía `bool, wait, error`) a `(Status, error)` de dest (#106)? | additive asked — Desired Tests mapea `TestTraefik12091_RedisUnreachable`; dest ya cambió la firma en #106 (In-scope / Current). | assumed — `status, err := limiter.Allow(...)`; `errors.Is(err, simpleredis.ErrUnreachable)`; fallar el test si `status.Allowed` es true. No reintroducir la tupla. `wait` no se afirma en outage (Status cero). | explore |
| ¿Los tests existentes que casan `err.Error() == RedisUnreachable` se reescriben a `errors.Is`? | additive incidental — no se cambia ningún contrato existente; 2 sitios enumerados (`tokenbucket/limiter_test.go:237`, `tokenbucket/status_test.go:178`; raíces `tokenbucket/**/*.go`) se dejan; ningún criterio pide reescribirlos. | assumed — no reescribir. El fichero 12091 es la prueba `errors.Is`. El matching por texto sigue siendo cierto y es control legado. | explore |

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
| Reviewed head | c7d3dc9381d3e7da4ec80857958218f798afd9b1 | Card must match the branch you measured |

### Stored data model
Ninguno.
