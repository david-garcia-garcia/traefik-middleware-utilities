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
Reviewed head: cbca6579
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
| Branch | 2026-11-22-backendbackoff-new-key-scan pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/122 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
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
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Qué costura hace que admitir una clave nueva sea O(1) amortizado en compilado y Yaegi, con el mismo comportamiento observable de `std_go_backendbackoff_allow`? | bounded asked — `loadEntry`/`dropExpired`/`dropOne` y `memEntry` viven solo en `backendbackoff` (1 def cada uno, 1 literal de producción, 0 constructores en tests; raíces `backendbackoff/*.go` y `**/*.go` del módulo); Desired nombra «O(1) amortized» y el mismo comportamiento del spec. | assumed — listed, not picked. Opciones 1–4 arriba; 1 tiene la evidencia del hermano #105; 3 no cumple el Desired; las vetadas no entran. | explore |


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
| Reviewed head | cbca65793934ae02ed59ff68e694fc784b89424d | Card must match the branch you measured |

### Stored data model
None.
