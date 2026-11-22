## Motivation
Cada `Allow` y `Report` del `Gate` serializa en un solo `g.mu` durante toda la llamada. El coste que crece con el mapa no está en una clave ya residente: está en admitir una clave que no está en `g.keys`.

Cuando `loadEntry` ve una clave desconocida, `dropExpired` recorre todo el mapa bajo ese mutex. En el tope de 65.536, `dropOne` lo recorre otra vez y borra un slot arbitrario. El coste por clave nueva crece con `len(g.keys)`: en compilado, de 1 µs (mapa vacío) a 1,8 ms (65.536); bajo Yaegi, de 36 µs a 5,1 ms (16.384). En Yaegi, unas 270 claves nuevas/s saturan el mutex (mapa a 16k, TTL 60 s); en compilado, cerca del tope, unas 500/s.

Todo `Allow` y `Report` de todos los backends de ese `Gate` se encola detrás del barrido. Solo pega cuando la clave tiene alta cardinalidad (cliente, IP, ruta dinámica); decenas o cientos de claves por backend no se notan. Es el mismo patrón que #105 ya sustituyó en `tokenbucket.Memory`.

Priority: P2 — dolor real de operador, con workaround o radio de impacto limitado

## Implementation
`memEntry` entra en una lista doblemente enlazada de expiración con punteros concretos (`key`, `prev`, `next`); `Gate` guarda `head` y `tail` (sin `container/list`). Cada escritura de `expireAt` — cada `Allow`, también denies, y cada `Report` que aterriza — mueve la entrada a la cola con `moveToBackLocked` (sin-op si ya es cola; reenlace in situ para no pagar una llamada extra bajo Yaegi). Una clave nueva llama `makeRoomLocked`: saca cabezas caducadas y, si sigue en el tope, saca la cabeza. `Close` anula `head`/`tail` con el mapa. El comportamiento observable sigue `std_go_backendbackoff_allow` (TTL perezoso, refresh en cada `Allow`, tope 65.536, caducadas primero, luego un slot arbitrario). Tests de lista y tope, más benches de clave nueva con tabla llena en compilado y Yaegi; se cierra la nota de deuda del barrido.

## What this changes
**Operators.** Con claves de alta cardinalidad, admitir una clave nueva ya no satura `g.mu` recorriendo el mapa; no hay clave de despliegue nueva.
**Admin users.** None.
**Developers.** Las firmas de `New`, `Allow`, `Report` y `Close` no cambian; admitir una clave nunca vista ya no recorre `g.keys`, y en el tope se sigue expulsando un slot arbitrario.
**End users.** None.

## Merge readiness
In progress. 0 items remain.

Priority: P2 — dolor real de operador, con workaround o radio de impacto limitado
Reviewed head: cea3ecab
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
| Branch | 2026-11-22-backendbackoff-new-key-scan pushed | `git` |
| OpenSpec | backendbackoff-expiry-list | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/122 | pr-host |
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
Ticket 2026-11-22-backendbackoff-new-key-scan on branch 2026-11-22-backendbackoff-new-key-scan targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/122; CI not seen.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-new-key-scan/devstate/2026/11/2026-11-22-backendbackoff-new-key-scan/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-new-key-scan/devstate/2026/11/2026-11-22-backendbackoff-new-key-scan/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-new-key-scan/devstate/2026/11/2026-11-22-backendbackoff-new-key-scan/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-new-key-scan/devstate/2026/11/2026-11-22-backendbackoff-new-key-scan/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-new-key-scan/devstate/2026/11/2026-11-22-backendbackoff-new-key-scan/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-new-key-scan/devstate/2026/11/2026-11-22-backendbackoff-new-key-scan/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-new-key-scan/devstate/2026/11/2026-11-22-backendbackoff-new-key-scan/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-new-key-scan/devstate/2026/11/2026-11-22-backendbackoff-new-key-scan/codereview_coverage.md) — 1 total, 0 pending, 0 completed, 1 skipped


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | cea3ecabd44e8925a378319ba7fb7db928f68713 | Card must match the branch you measured |

### Stored data model
None.
