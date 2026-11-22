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
Reviewed head: f7e69177
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
| Branch | 2026-11-22-tokenbucket-concurrent-first-requests pushed | `git` |
| OpenSpec | tokenbucket-concurrent-first-requests | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/110 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-concurrent-first-requests/openspec/changes/tokenbucket-concurrent-first-requests/proposal.md) — modified
- [std_go_tokenbucket_lua-eval](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-concurrent-first-requests/openspec/changes/tokenbucket-concurrent-first-requests/proposal.md) — modified


## Deviations from the ask
- taken: misma puerta que `TestLive_Limiter` vía `runForEachLiveEngine` / `waitLiveClient`, y PoolSize 16 para que las 16 peticiones se solapen. → `runForEachLiveEngine` para skip y tabla Redis/Dragonfly; `simpleredis.New` en `traefik_issue14328_e2e_test.go` con PoolSize 16, MaxIdleConns 16, PoolTimeout 2s, más espera local `waitLiveClient14328`. — `tokenbucket/limiter_e2e_test.go (waitLiveClient)` — honrar las dos letras a la vez añade un parámetro PoolSize a `waitLiveClient`, un helper cuyo trabajo es esperar con Config por defecto (PoolSize 8). Out of scope prohíbe helpers compartidos; el repro 14328 ya construye el cliente con PoolSize 16.. Requester: not asked.


## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-tokenbucket-concurrent-first-requests on branch 2026-11-22-tokenbucket-concurrent-first-requests targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/110; CI not seen.

## Explore Decisions
None.

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
| Reviewed head | f7e69177c023220f491acfad32f403840fc741a9 | Card must match the branch you measured |

### Stored data model
None.
