## Motivation
`Gate.Allow` comprobaba `context.Context` solo para devolver `ctx.Err()` antes de tomar el mutex. Eso acopla la máquina de estados síncrona (crédito, OPEN/HALF-OPEN/CLOSED) al ciclo de vida del request HTTP, duplicando una decisión que el middleware ya puede tomar antes de intentar admisión. Mientras el contrato vivo y los devdocs exigían pasar `req.Context()` y negar en cancelación, cualquier caller que quisiera una admisión puramente local debía arrastrar contexto sin beneficio para la lógica interna.

Priority: P3 — contrato público y documentación; sin daño operativo inmediato en este árbol (solo el paquete `backendbackoff` importa la API aquí).

## Implementation
Se cambió la firma a `Allow(key string)`, se eliminó el early return por contexto y se actualizaron tests, probe Yaegi, benchmark y devdocs. El delta OpenSpec en `openspec/changes/backendbackoff-allow-nocontext/` sustituye el escenario de cancelación interna por responsabilidad del caller antes de llamar; `Report` sigue sin contexto. La spec del catálogo vivo se fusionará en archive.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** BREAKING: invocar `Allow(key)` sin contexto; comprobar cancelación o deadline del request antes de llamar; no esperar `context.Canceled` ni `DeadlineExceeded` desde `Allow`.
**End users.** None.

## Merge readiness
Ready for review. 0 items remain.

Priority: P3 — contrato público y documentación; sin daño operativo inmediato en este árbol (solo el paquete `backendbackoff` importa la API aquí).
Reviewed head: 4624c71
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/38923235017 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-10-16-backendbackoff-allow-nocontext pushed | `git` |
| OpenSpec | backendbackoff-allow-nocontext | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/101 | pr-host |
| CI | build 38923235017 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/38923235017 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/38923235017 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_backendbackoff_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/openspec/changes/backendbackoff-allow-nocontext/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-10-16-backendbackoff-allow-nocontext on branch 2026-10-16-backendbackoff-allow-nocontext targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/101; CI build 38923235017 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/38923235017.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Existen plugins Traefik u otros consumidores fuera de este repositorio que sigan en `Allow(ctx, key)` y deban migrarse en este PR? | additive asked — no reshape en árbol; **Unknowns** en `requirement.md` nombra la incertidumbre sin pedir migración externa | assumed — fuera de alcance de este repo; el cambio de firma es breaking a propósito; no bloquear implement; quien despliegue copias externas migra en su propio ciclo. | explore |


## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/devstate/2026/10/2026-10-16-backendbackoff-allow-nocontext/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/devstate/2026/10/2026-10-16-backendbackoff-allow-nocontext/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/devstate/2026/10/2026-10-16-backendbackoff-allow-nocontext/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/devstate/2026/10/2026-10-16-backendbackoff-allow-nocontext/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/devstate/2026/10/2026-10-16-backendbackoff-allow-nocontext/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/devstate/2026/10/2026-10-16-backendbackoff-allow-nocontext/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/devstate/2026/10/2026-10-16-backendbackoff-allow-nocontext/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-10-16-backendbackoff-allow-nocontext/devstate/2026/10/2026-10-16-backendbackoff-allow-nocontext/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 4624c71d6a7b824b06601623263384f51e2d7c9b | Card must match the branch you measured |

### Stored data model
None.
