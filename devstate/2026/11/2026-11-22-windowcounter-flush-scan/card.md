## Motivation
El `Limiter` de `windowcounter` en modo buffered (`syncRate > 0`) acumula deltas en `l.windows` y un `AfterFunc` (`flushAfterTick`) los envía a Redis. #105 ya agrupa esos EVAL para no sujetar `l.mu` durante Redis. Take y Peek del mismo `Limiter` siguen serializando en ese mutex; `Sleep` y `Close` llaman al mismo flush.

Cada tick recorre todo `l.windows` dos veces con `l.mu` cogido: recoger ventanas con `localDelta > 0` y podar las caducadas. Ambos pasos son O(K) aunque no haya nada sucio ni que caduque. El mapa no tiene tope; K ≈ claves distintas de las últimas 2–3 ventanas (`expireAt = windowStart + 2*window`). El limitador por IP con rotación de direcciones infla K. El timer se rearma al volver del flush, así que el duty cycle del mutex es `scan / (scan + syncRate)`.

Un tick idle (nada que enviar) mide ~11,1 ms compilado y ~51,9 ms bajo Yaegi a 200 000 ventanas vivas, y ~67 ms compilado a 1 000 000. Con `syncRate` 20 ms y 200 000 ventanas, Yaegi deja el mutex ocupado ~72 % del tiempo: cada Take/Peek de ese `Limiter`, de cualquier clave, espera detrás del scan (del orden de 20 ms de más, peor ~52 ms). Lo mismo pagan `Sleep` y `Close`.

Si se deja así, un `Limiter` buffered con muchas claves vivas atasca toda la admisión de esa instancia en cada tick, compilado y como plugin Traefik (Yaegi). No cambia el resultado de admisión; sí el stall, que crece con el número de ventanas vivas.

Priority: P2 — dolor real de operador y usuario final, con radio de impacto limitado

## Implementation
Un índice sucio `dirty map[string]*windowState` guarda las ventanas con `localDelta > 0` (o `flushDelta > 0` mientras un flush está en vuelo). `takeBuffered` las mete solo en la transición 0→1; `applyFlushOutcomeLocked` las saca cuando ambos deltas vuelven a 0. `flushPendingLocked` recorre `l.dirty` en vez de `l.windows`.

Un índice de caducidad `expiry map[int64]map[string]*windowState` agrupa por segundo de `expireAt`. Tres helpers (`putWindowLocked`, `setExpireLocked`, `deleteWindowLocked`) son el único sitio que inserta, mueve o borra en `l.windows` y en los cubos. La poda visita solo cubos vencidos (`expireAt == 0` o `nowUnix >= expireAt`) y borra ventanas idle con la misma condición de siempre.

El coste del tick queda en O(sucias + que caducan ahora + segundos distintos de `expireAt`). Admisión, semántica de flush (`std_go_windowcounter_sync-flush`) y batching no cambian. Un test aleatorio comprueba índices y conteos Redis; otro afirma que el flush idle a 200 000 ventanas queda por debajo de 3× el de 10 000. Hay `BenchmarkFlushIdle` compilado y Yaegi.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** None.
**End users.** None.

## Merge readiness
In progress. 0 items remain.

Priority: P2 — dolor real de operador y usuario final, con radio de impacto limitado
Reviewed head: de435f5c
Owner decision: None.

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
| Branch | 2026-11-22-windowcounter-flush-scan pushed | `git` |
| OpenSpec | windowcounter-flush-dirty-index | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/120 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-windowcounter-flush-scan on branch 2026-11-22-windowcounter-flush-scan targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/120; CI not seen.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-windowcounter-flush-scan/devstate/2026/11/2026-11-22-windowcounter-flush-scan/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-windowcounter-flush-scan/devstate/2026/11/2026-11-22-windowcounter-flush-scan/codereview_nitpicks.md) — 2 total, 0 pending, 2 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-windowcounter-flush-scan/devstate/2026/11/2026-11-22-windowcounter-flush-scan/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-windowcounter-flush-scan/devstate/2026/11/2026-11-22-windowcounter-flush-scan/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-windowcounter-flush-scan/devstate/2026/11/2026-11-22-windowcounter-flush-scan/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-windowcounter-flush-scan/devstate/2026/11/2026-11-22-windowcounter-flush-scan/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-windowcounter-flush-scan/devstate/2026/11/2026-11-22-windowcounter-flush-scan/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-windowcounter-flush-scan/devstate/2026/11/2026-11-22-windowcounter-flush-scan/codereview_coverage.md) — 3 total, 0 pending, 2 completed, 1 skipped


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | de435f5c559a034619554aedfee2e8e5fb4669cf | Card must match the branch you measured |

### Stored data model
None.
