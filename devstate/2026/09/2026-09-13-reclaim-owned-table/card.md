Developer review: in progress — 2026-09-13T08:14:27Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `reclaim-owned-table` folds `std_go_reclaim_context-lease` off the process table onto `New(Config)` and caller-owned lifetime.

**End users.** None.

## Motivation
On dest, reclaim still hands every caller the same process table (`Default` and package `Open`). Grace is already a constructor argument on `NewTable` and is never written after create, but the singleton is born at `DefaultGrace`. The only way to change it is `ResetWith`, which replaces that one table for the whole process.

Two plugins that want different grace, or that should not share keys, cannot both be right on that table. This ticket drops the singleton so each caller creates a table through `New()`, feeds constructor config there, and owns lifetime.

If this stays unmerged, production-shaped callers (`e2e/reclaimprobe`, README, usage) keep teaching a process-wide table that cannot take per-instance config.

```mermaid
flowchart LR
  pluginA["Plugin A wants grace 0"] --> defaultTable["Default / package Open"]
  pluginB["Plugin B wants grace 10s"] --> defaultTable
  defaultTable --> oneGrace["One table, DefaultGrace"]
```

## Merge readiness
Propose is apply-ready (`reclaim-owned-table`). Product apply has not started. Implement is next.

Priority: P3 — spec and internal API clarity, no current operator or end-user harm
Reviewed head: b228dcd
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Head pushed; CI on this SHA not seen |
| CI proof | 1/6 | `get_status` pending on previous head; this SHA just pushed |
| Local tests proof | N/A | Before implement (`localTests: none`) |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-owned-table pushed | `git push` `b228dcd` |
| OpenSpec | reclaim-owned-table | `openspec/changes/reclaim-owned-table/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/65 | pr-host List |
| CI | not seen | pr-host get_status pending on prior SHA; `b228dcd` just pushed |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
- [std_go_reclaim_context-lease](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/openspec/changes/reclaim-owned-table/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-reclaim-owned-table` from `origin/master`. Stub PR #65. Change `reclaim-owned-table` is apply-ready. Next is implement.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Is `New` `New(grace time.Duration)` or `New(Config)` with `Grace` as a field? | bounded asked | assumed — `New(Config)` by value; `Grace` only; copy at `New`; no setter; negative → `DefaultGrace`; zero stays 0. Delete `NewTable`. | explore |
| Is `reclaim/default.go` deleted or left without the singleton? | bounded asked | assumed — delete `reclaim/default.go`. | explore |
| How does `e2e/reclaimprobe` hold the table so two Traefik `New`s still share an incarnation within grace? | bounded asked | assumed — package-level `var` table in `reclaimprobe`, `New(Config{Grace: DefaultGrace})` once; plugin `New` calls `table.Open`. | explore |
| Does `Config` live in `table.go` or a new `config.go`? | additive asked | assumed — `Config` next to `Table` in `reclaim/table.go`. No `reclaim/config.go`. | explore |

## Before merge
- [ ] Remove `Default`, package `Open`, `Reset`, and `ResetWith`; construct tables with `New(Config)` and caller-owned lifetime
- [ ] Fold spec, usage, README, and `reclaimprobe` off the process table

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | b228dcd9fe666e4ec53ec1d4c815c578b9a8de15 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: not applied yet; dest still exposes the process table. The change artifacts specify `New(Config)` and deleting `default.go`.

Do we have a high-confidence way to reproduce? Yes — `reclaim/default.go` plus `TestDefault_*` in `reclaim/table_test.go`.

Is this the best way to solve the issue? Yes versus dest — remove the singleton rather than add a second config path on `Default`.

### Evidence
What I checked:
- `openspec validate reclaim-owned-table --type change --strict` valid (`openspec`, `b228dcd`)
- `origin/master...HEAD` product paths: `openspec/changes/reclaim-owned-table/` only (`git diff`)
- OPEN PR #65 (GitHub MCP)
- CI: not seen on `b228dcd` (just pushed)

### Rank-up moves
None.
