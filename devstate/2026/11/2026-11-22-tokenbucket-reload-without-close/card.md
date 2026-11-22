## Summary
Test live de 50 recargas sin Close: cada una construye un cliente simpleredis y un limitador Redis a través de un proxy TCP que cuenta sockets, llama Allow una vez y descarta el cliente sin Close. Tras GC, `proxy.open == 0` y no sobreviven goroutines de cliente (`leakedGoroutines > 2` falla). Sin cambio de producto. Redis y Dragonfly en verde.

La existencia del defecto se tomó de estos informes upstream:
- Issue https://github.com/traefik/traefik/issues/13618
- PR upstream https://github.com/traefik/traefik/pull/13619 (abierto)

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
Ready for review. 0 items remain.

Priority: unknown — motivation not written
Reviewed head: ed8537bd
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996420726 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-tokenbucket-reload-without-close pushed | `git` |
| OpenSpec | tokenbucket-reload-without-close | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/113 | pr-host |
| CI | build 44996420726 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996420726 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996420726 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-reload-without-close/openspec/changes/tokenbucket-reload-without-close/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-tokenbucket-reload-without-close on branch 2026-11-22-tokenbucket-reload-without-close targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/113; CI build 44996420726 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996420726.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Se conserva el umbral `leakedGoroutines > 2` del repro o se ajusta en dest? | additive asked — Tests pide conservar casos de control; Unknowns deja el umbral a implement al transcribir | assumed — transcribir `leakedGoroutines > 2`. Hoy el repro midió +0. Implement solo lo aprieta con evidencia dest; no lo afloja. | explore |


## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | ed8537bd54a3f06ef0c053d471425a054cb937d2 | Card must match the branch you measured |

### Stored data model
None.
