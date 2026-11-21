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
Reviewed head: 578cbaff
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44890987232 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-21-ratelimit-benchmarks pushed | `git` |
| OpenSpec | ratelimit-benchmarks-key-scaling | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/105 | pr-host |
| CI | build 44890987232 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44890987232 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44890987232 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-benchmarks/openspec/changes/ratelimit-benchmarks-key-scaling/proposal.md) — modified
- [std_go_windowcounter_sync-flush](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-benchmarks/openspec/changes/ratelimit-benchmarks-key-scaling/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
- [ ] [Align the `std_go_simpleredis_resp-commands` rationale with "compiled and under Yaegi"](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-benchmarks/knowledge/debt/2026-11-21-simpleredis-spec-compiled-only-rationale.md) — `tokenbucket.Memory` recorre todos los buckets bajo su mutex en cada fuente nueva (≈0.8–1.6 ms con 60k–65k fuentes). El solicitante lo metió en este cambio el 2026-11-21 (respuesta 3). Cuando implement lo resuelva: `[x]`, `Taken:` y borrar el fichero de deuda.


## How this fits together
Ticket 2026-11-21-ratelimit-benchmarks on branch 2026-11-21-ratelimit-benchmarks targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/105; CI build 44890987232 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44890987232.

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
| Reviewed head | 578cbaffe8aa6adf74ce7db2545c21687ea0d019 | Card must match the branch you measured |

### Stored data model
None.
