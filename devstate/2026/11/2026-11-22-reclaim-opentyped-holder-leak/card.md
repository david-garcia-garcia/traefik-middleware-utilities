## Motivation
OpenTyped pide el valor de una clave de Table ya tipado como T. El bind ocurre primero, por el mismo camino que OpenWithHooks: incrementa holders y, si el ctx del llamante tiene Done, registra un AfterFunc hacia drop. Recién después hace stored.(T).

En un mismatch devolvía el zero T y el error `reclaim: open %q: want %T, got %T` sin soltar ese holder. El llamante no obtuvo valor y su ctx seguía manteniendo la encarnación despierta. Cada reintento con el mismo ctx sumaba otro holder y otro AfterFunc. Cien OpenTyped[int] fallidos sobre una clave cuyo create guarda un string dejan holders=100 y Close no corre con Grace 0. El escenario live de mismatch y el test de error solo pedían zero y el texto; no medían holders ni Close.

Si se deja así, Sleep y Close no pueden correr mientras viva ese ctx. Un retry o un ctx largo clava el valor y acumula watchers. El disparo realista es dos consumidores que comparten una Table y la misma clave con tipos distintos, o un error de tipo en el llamante.

Priority: P2 — fuga real de holders y Close que no corre, radio limitado al mismatch de OpenTyped

## Implementation
El camino interno de bind ahora devuelve un bindLease (clave, encarnación, stop del AfterFunc, marca dropped). OpenWithHooks descarta el lease y conserva su firma. OpenTyped, si el assert falla, llama releaseLease y sigue devolviendo el mismo error. OpenTyped sigue siendo una call expression; no hay forma genérica nueva.

releaseLease para el AfterFunc cuando hay stop y luego dropOnce. dropOnce, bajo el mutex de la tabla, marca el lease y corre drop a lo sumo una vez, de modo que el release y el watcher (AfterFunc o watch con Done nil) no pueden soltar el mismo holder dos veces. finishBind con el ctx ya hecho sigue dropeando al momento y no entrega lease. Un create que ya publicó no se deshace: solo se va el holder de esta llamada; si no queda otro, el drop normal duerme o cierra a grace 0.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** Un OpenTyped con tipo incorrecto ya no deja al llamante como holder: el recuento vuelve atrás, su ctx no mantiene el valor, y si era el último holder Close corre a grace 0 aunque el ctx siga vivo. Firma, texto de error y el OpenTyped con éxito no cambian; Open y OpenWithHooks tampoco.
**End users.** None.

## Merge readiness
Ready for review. 0 items remain.

Priority: P2 — fuga real de holders y Close que no corre, radio limitado al mismatch de OpenTyped
Reviewed head: 893882df
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/45013740682 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-reclaim-opentyped-holder-leak pushed | `git` |
| OpenSpec | reclaim-opentyped-mismatch-release | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/123 | pr-host |
| CI | build 45013740682 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/45013740682 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/45013740682 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_reclaim_value-lifecycle](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/openspec/changes/archive/2026-11-22-reclaim-opentyped-mismatch-release/proposal.md) — modified

Completed:
- [std_go_reclaim_value-lifecycle](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/openspec/specs/std_go_reclaim_value-lifecycle/spec.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-reclaim-opentyped-holder-leak on branch 2026-11-22-reclaim-opentyped-holder-leak targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/123; CI build 45013740682 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/45013740682.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/devstate/2026/11/2026-11-22-reclaim-opentyped-holder-leak/codereview_standards.md) — 1 total, 0 pending, 1 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/devstate/2026/11/2026-11-22-reclaim-opentyped-holder-leak/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/devstate/2026/11/2026-11-22-reclaim-opentyped-holder-leak/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/devstate/2026/11/2026-11-22-reclaim-opentyped-holder-leak/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/devstate/2026/11/2026-11-22-reclaim-opentyped-holder-leak/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/devstate/2026/11/2026-11-22-reclaim-opentyped-holder-leak/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/devstate/2026/11/2026-11-22-reclaim-opentyped-holder-leak/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-reclaim-opentyped-holder-leak/devstate/2026/11/2026-11-22-reclaim-opentyped-holder-leak/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 893882df4600b8f5f0e7b9628e5aa71b6fb5847e | Card must match the branch you measured |

### Stored data model
None.
