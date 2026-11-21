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
Reviewed head: 6759ce9f
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44888631699 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-21-simpleredis-bug-yaegi-select pushed | `git` |
| OpenSpec | simpleredis-drop-select-under-yaegi | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/104 | pr-host |
| CI | build 44888631699 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44888631699 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44888631699 |
| Local tests | passed | handoff.yaml localTests |
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
Ticket 2026-11-21-simpleredis-bug-yaegi-select on branch 2026-11-21-simpleredis-bug-yaegi-select targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/104; CI build 44888631699 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44888631699.

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
| Reviewed head | 6759ce9fa6ec008534e1ee886ede37fa2de5ab86 | Card must match the branch you measured |

### Stored data model
None.
