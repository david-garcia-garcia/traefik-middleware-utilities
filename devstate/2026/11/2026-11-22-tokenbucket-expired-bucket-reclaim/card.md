## Motivation
Quien ya conoce el limitador in-memory sabe que hay un bucket por fuente opaca, un tope de 65536 y una lista de caducidad; el camino Redis termina cada EVAL con `EXPIRE` en segundos enteros. El síntoma que origina este trabajo está en Traefik: https://github.com/traefik/traefik/issues/13704. Allí `ttlmap` solo libera una clave caducada si alguien la vuelve a consultar, o cuando el mapa llega a 65536 entradas y entonces libera solo una; la memoria crece con el número de fuentes distintas vistas, no con las activas.

Ese fallo no se da en este árbol: Memory ya no retiene buckets caducados cuando entra una fuente nueva, y Redis/Dragonfly ya ponen caducidad en cada escritura. Lo que faltaba era la prueba. Los tests del tope 65536 cubren el mapa lleno, no diez fuentes ociosas más allá de `ttl` más una fuente nueva bajo el tope. El e2e vivo admite y deniega, pero no lee el `PTTL` de la clave del bucket.

Sin ese ancla, un cambio posterior en la purga al alta o en el `EXPIRE` del script podría volver a retener fuentes ociosas —Memory creciendo con cada clave distinta, claves Redis sin TTL— y ni el tope 65536 ni el admit/deny lo detectarían.

Priority: P3 — tests y claridad de spec, sin daño actual a usuarios u operadores

## Implementation
Se ancla el comportamiento que el árbol ya tiene, sin tocar `Memory.Allow`, `makeRoomLocked` ni el `EXPIRE` del EVAL.

En Memory, un reloj de test congelado admite diez claves sintéticas, salta `ttl+9s`, admite `"fresh"` y comprueba bajo el mutex que el mapa guarda exactamente un bucket, el de la clave nueva.

En Redis y Dragonfly reales, el mismo gating que el e2e ya existente (`TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`) admite tres veces con `ttl` 7s y lee `PTTL` por EVAL sobre `KEYS[1]`; el valor debe quedar en `(0, ttl]`. Los helpers van en el propio fichero e2e con sufijo `13704`.

El catálogo vivo recibe un escenario bajo el requisito de tope y caducadas: diez fuentes ociosas más una nueva dejan una fuente almacenada. No hay SHALL nuevo de `PTTL`; el `EXPIRE` entero ya estaba.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** Quien mantenga el almacén Memory o el EVAL Redis debe conservar que diez fuentes ociosas más allá de `ttl` más una nueva dejen exactamente una fuente, y que la clave del bucket tenga `PTTL` en `(0, ttl]` tras `Allow`.
**End users.** None.

## Merge readiness
Ready for review. 0 items remain.

Priority: P3 — tests y claridad de spec, sin daño actual a usuarios u operadores
Reviewed head: be24d750
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996123943 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-tokenbucket-expired-bucket-reclaim pushed | `git` |
| OpenSpec | tokenbucket-expired-bucket-reclaim | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/111 | pr-host |
| CI | build 44996123943 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996123943 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996123943 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/openspec/changes/tokenbucket-expired-bucket-reclaim/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-tokenbucket-expired-bucket-reclaim on branch 2026-11-22-tokenbucket-expired-bucket-reclaim targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/111; CI build 44996123943 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996123943.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Cómo adaptar los tests de referencia a `Allow(ctx, key) (Status, error)` sin copiar helpers de otros issues? | additive asked — ficheros de test que este cambio crea; la línea Tests del Design nombra ficheros autocontenidos con sufijo 13704 | assumed — llamar `Allow` como `(status, err)` y fallar si `err != nil`. El test Memory no usa `countAdmits`; cuenta `len(limiter.buckets)` bajo el mutex. El e2e declara `liveKey13704` y `evalKey13704` en `traefik_issue13704_e2e_test.go` (Eval `pttl` con `ScriptSHA1Hex`, KEYS[1] para Dragonfly). No copiar `liveKey`/`evalKey` del árbol de repros ni de otros issues. | propose |


## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/devstate/2026/11/2026-11-22-tokenbucket-expired-bucket-reclaim/codereview_standards.md) — 1 total, 0 pending, 1 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/devstate/2026/11/2026-11-22-tokenbucket-expired-bucket-reclaim/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/devstate/2026/11/2026-11-22-tokenbucket-expired-bucket-reclaim/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/devstate/2026/11/2026-11-22-tokenbucket-expired-bucket-reclaim/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/devstate/2026/11/2026-11-22-tokenbucket-expired-bucket-reclaim/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/devstate/2026/11/2026-11-22-tokenbucket-expired-bucket-reclaim/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/devstate/2026/11/2026-11-22-tokenbucket-expired-bucket-reclaim/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/devstate/2026/11/2026-11-22-tokenbucket-expired-bucket-reclaim/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | be24d75041b70872ce193d8c7f8ce366f338a488 | Card must match the branch you measured |

### Stored data model
None.
