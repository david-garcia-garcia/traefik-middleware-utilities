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
In progress. 1 items remain.

Priority: unknown — motivation not written
Reviewed head: 04e0b469
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
| Branch | 2026-11-21-simpleredis-bug-yaegi-select pushed | `git` |
| OpenSpec | simpleredis-drop-select-under-yaegi | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/104 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_simpleredis_resp-commands](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-bug-yaegi-select/openspec/changes/simpleredis-drop-select-under-yaegi/proposal.md) — modified
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-bug-yaegi-select/openspec/changes/simpleredis-drop-select-under-yaegi/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
- [ ] [Replace the `select` in reclaim `waitCtx` and guard every production package against `select`](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-bug-yaegi-select/knowledge/debt/2026-11-21-reclaim-waitctx-select.md) — reclaim `waitCtx` is the last production `select` after this change, and the AST guard covers only simpleredis.


## How this fits together
Ticket 2026-11-21-simpleredis-bug-yaegi-select on branch 2026-11-21-simpleredis-bug-yaegi-select targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/104; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Qué sustituye al canal `inUseTurns` y a sus dos `select` (fast take, espera saturada, free) sin perder ni duplicar turnos y sin regresión en el camino caliente bajo Yaegi? | bounded asked — cambia la unidad existente `inUseTurns` con call sites enumerados (2 archivos de producción, 5 de test + 1 sonda, 12+2 líneas de spec; raíces: todo el worktree salvo devstate) y los migra aquí; requirement.md «Deseado» 1 y 2 y Unknowns «Diseño concreto del sustituto de cada select» | assumed — mutex + contador + `turnCap` + cola FIFO de waiters; fast take y free inline sin método nuevo ni `defer`; espera saturada con `context.WithTimeout` + un `context.AfterFunc` y recepción simple. Si en implement el par `WithTimeout`+`AfterFunc` mide peor que `AfterFunc`×2 sin `sync.Once`, se usa este último. | explore |


## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 04e0b4697bf1f9e6ca3c31ee8d3d24a085f9e92f | Card must match the branch you measured |

### Stored data model
None.
