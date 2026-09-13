Developer review: in progress — 2026-09-13T08:03:28Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

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
Prepare is grounded (`qualified-with-gaps`). Product apply has not started. Explore is next.

Priority: P3 — spec and internal API clarity, no current operator or end-user harm
Reviewed head: 771d804
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI in progress; no product apply yet |
| CI proof | 3/6 | Checks in progress on the stub PR |
| Local tests proof | N/A | Before implement (`localTests: none`) |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-owned-table pushed | `git push` |
| OpenSpec | none | no change folder |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/65 | pr-host Create |
| CI | build 34746772807 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34746772807 | pr-host CI |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-reclaim-owned-table` from `origin/master` (reclaim present; `origin/HEAD` is stale `initial`). Stub PR #65 is the durable card. Next is explore.

## Explore Decisions
None.

## Before merge
- [ ] Remove `Default`, package `Open`, `Reset`, and `ResetWith`; construct tables with `New()` and caller-owned lifetime
- [ ] Fold spec, usage, README, and `reclaimprobe` off the process table

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 771d804260d2115ba7e93a15e83ba233d0b18f88 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: not applied yet; dest still exposes the process table.

Do we have a high-confidence way to reproduce? Yes — `reclaim/default.go` plus `TestDefault_*` in `reclaim/table_test.go`.

Is this the best way to solve the issue? Yes versus dest — remove the singleton rather than add a second config path on `Default`.

### Evidence
What I checked:
- `git ls-tree origin/master reclaim` exists; `origin/initial` has no `reclaim/`
- `reclaim/default.go`, `reclaim/table.go` `NewTable`, spec `std_go_reclaim_context-lease`, `e2e/reclaimprobe/plugin.go`, usage docs (`git`, dest `24a6852`)
- OPEN PR #65, empty comment inventory (GitHub MCP)
- CI run 34746772807 in progress on `771d804` (start commit `950cb96` had succeeded as 34746615300)

### Rank-up moves
None.
