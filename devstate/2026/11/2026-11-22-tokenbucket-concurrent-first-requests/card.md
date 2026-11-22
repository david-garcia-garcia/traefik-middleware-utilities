## Summary
Tests de regresión (sin cambio de producto) para primeras peticiones concurrentes de una fuente nueva. El defecto de Traefik https://github.com/traefik/traefik/issues/14328 (mencionado primero en https://github.com/traefik/traefik/issues/13957) no ocurre aquí: Memory sostiene un mutex durante lookup, create y consume; Redis ejecuta un script atómico. No hay PR upstream de #14328.

## Motivation
Not yet.

## Implementation
Not yet.

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Merge readiness
In progress. 0 items remain.

Priority: unknown — motivation not written
Reviewed head: 06a46c89
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
| Branch | 2026-11-22-tokenbucket-concurrent-first-requests pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/110 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
- taken: misma puerta que `TestLive_Limiter` vía `runForEachLiveEngine` / `waitLiveClient`, y PoolSize 16 para que las 16 peticiones se solapen. → `runForEachLiveEngine` para skip y tabla Redis/Dragonfly; `simpleredis.New` en `traefik_issue14328_e2e_test.go` con PoolSize 16, MaxIdleConns 16, PoolTimeout 2s, más espera local `waitLiveClient14328`. — `tokenbucket/limiter_e2e_test.go (waitLiveClient)` — honrar las dos letras a la vez añade un parámetro PoolSize a `waitLiveClient`, un helper cuyo trabajo es esperar con Config por defecto (PoolSize 8). Out of scope prohíbe helpers compartidos; el repro 14328 ya construye el cliente con PoolSize 16.. Requester: not asked.


## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-tokenbucket-concurrent-first-requests on branch 2026-11-22-tokenbucket-concurrent-first-requests targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/110; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Cómo obtener PoolSize 16 sin cambiar `waitLiveClient` (PoolSize por defecto 8)? | additive asked — test e2e nuevo que este cambio crea; Desired nombra PoolSize 16 y la puerta viva `runForEachLiveEngine` / `waitLiveClient` | assumed — `runForEachLiveEngine` para skip/tabla de motores; cliente en el archivo nuevo con PoolSize 16, MaxIdleConns 16, PoolTimeout 2s (como el repro). Espera de readiness en `waitLiveClient14328` local. No cambiar `waitLiveClient` (4 llamadas en `tokenbucket/`, buscado `waitLiveClient(` en el repo). | explore |
| ¿Qué hacer si un run posterior admitiera más de burst? | additive asked — Out of scope: no cambiar `memory.go` / `redis.go` / `lua.go`; Desired TEST ONLY | assumed — parar (`blocked`); no inventar arreglo de producto. Hoy dest no supera burst. | explore |
| ¿Hay que añadir un escenario al spec vivo de 16 primeras peticiones concurrentes? | additive incidental — contrato existente `std_go_tokenbucket_allow` / `lua-eval` sin ese escenario; Affected no lista specs; es medio para alinear el contrato con los tests | assumed — propose añade un escenario en `std_go_tokenbucket_allow` (mutex, admits ≤ burst) y uno en `std_go_tokenbucket_lua-eval` (Eval atómico, misma carga). Sin carpeta spec nueva. | explore |
| ¿TTL 120 s (repro) o el TTL de Traefik para 1 req/min (~61 s)? | additive asked — tests nuevos; Desired copia la carga del repro (1 req/min, burst 5, maxDelay 500 ms) | assumed — ttl 120 s como el repro, para no mezclar caducidad #13957 en este ticket. | explore |
| ¿Hace falta un test Yaegi de este issue? | additive incidental — `std_go_test-suites` documenta Yaegi para escenarios Allow; el requirement no nombra un `*_yaegi*` de 14328 | assumed — no. Compiled unit + e2e vivo. Bound the ask. | explore |
| ¿Además de PoolSize 16, hay que fijar MaxIdleConns y PoolTimeout? | additive asked — Desired nombra PoolSize 16; el repro también pone MaxIdleConns = concurrent y PoolTimeout 2s | assumed — igual que el repro: PoolSize 16, MaxIdleConns 16, PoolTimeout 2s. Idle trim por defecto 8 recortaría el pool; PoolTimeout 200 ms por defecto puede convertir solapes en `redis:unreachable`. | explore |

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 06a46c89b389377a650a84d29d5da51e1d7db939 | Card must match the branch you measured |

### Stored data model
None.
