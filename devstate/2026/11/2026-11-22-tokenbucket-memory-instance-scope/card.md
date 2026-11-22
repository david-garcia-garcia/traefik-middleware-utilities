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
Reviewed head: 56a562ce
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
| Branch | 2026-11-22-tokenbucket-memory-instance-scope pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/109 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
- [ ] [Consumer should share one Memory limiter per middleware name](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-memory-instance-scope/knowledge/debt/2026-11-22-tokenbucket-consumer-limiter-scope.md) — el arreglo de ámbito (un `*Memory` compartido por nombre de middleware, mismo prefijo Redis) pertenece al middleware consumidor, no a esta librería. Origen: https://github.com/traefik/traefik/issues/13706 ; PRs https://github.com/traefik/traefik/pull/13823 y https://github.com/traefik/traefik/pull/13707.


## How this fits together
Ticket 2026-11-22-tokenbucket-memory-instance-scope on branch 2026-11-22-tokenbucket-memory-instance-scope targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/109; CI not seen.

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
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 56a562ce227ea9d19091da6d87e6fd9121636e82 | Card must match the branch you measured |

### Stored data model
None.
