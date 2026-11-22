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
Ready for review. 1 items remain.

Priority: unknown — motivation not written
Reviewed head: a2a2ad1a
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996465867 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-tokenbucket-memory-instance-scope pushed | `git` |
| OpenSpec | tokenbucket-memory-instance-scope | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/109 | pr-host |
| CI | build 44996465867 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996465867 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996465867 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/openspec/changes/tokenbucket-memory-instance-scope/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
- [ ] [Consumer should share one Memory limiter per middleware name](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/knowledge/debt/2026-11-22-tokenbucket-consumer-limiter-scope.md) — el arreglo de ámbito (un `*Memory` compartido por nombre de middleware, mismo prefijo Redis) pertenece al middleware consumidor, no a esta librería. Origen: https://github.com/traefik/traefik/issues/13706 ; PRs https://github.com/traefik/traefik/pull/13823 y https://github.com/traefik/traefik/pull/13707.


## How this fits together
Ticket 2026-11-22-tokenbucket-memory-instance-scope on branch 2026-11-22-tokenbucket-memory-instance-scope targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/109; CI build 44996465867 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996465867.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿El catálogo vivo necesita un requisito de ámbito (Memory por instancia, Redis por clave) o basta GoDoc + tests? | additive asked — Desired «El ámbito pasa a ser contrato documentado»; binding «Make the scope part of the documented contract»; Affected nombra `openspec/specs/std_go_tokenbucket_allow/spec.md`; el delta añade una promesa, los callers existentes siguen valiendo | assumed — ADDED en `std_go_tokenbucket_allow`: cada `*Memory` es su propio espacio de claves; cualquier `*Redis` sobre la misma `key` comparte el estado del servidor; el prefijo sigue siendo del llamante. GoDoc es la superficie encargada; el catálogo registra la misma promesa. No nueva familia. | explore |


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
| Reviewed head | a2a2ad1a28924e871598a54e24c08951f2271e11 | Card must match the branch you measured |

### Stored data model
None.
