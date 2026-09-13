Developer review: in progress — 2026-09-13T08:18:34Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `reclaim` no longer has `Default` / package `Open` / `Reset` / `ResetWith`. Callers create a table with `New(Config)` (Grace copied and frozen). `e2e/reclaimprobe` holds a package-level table. README and `std_go_reclaim` / `std_go_backendbackoff` usage follow.

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
Apply landed. Local `go test ./reclaim/` passed. CI on this SHA not seen. Code review is next.

Priority: P3 — spec and internal API clarity, no current operator or end-user harm
Reviewed head: d4f8346
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Head pushed; CI on this SHA not seen |
| CI proof | 1/6 | `get_status` pending, 0 statuses on `d4f8346` |
| Local tests proof | 6/6 | `localTests: passed` (`go test -count=1 -timeout 60s ./reclaim/`) |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-owned-table pushed | `git push` `d4f8346` |
| OpenSpec | reclaim-owned-table | `openspec/changes/reclaim-owned-table/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/65 | pr-host List |
| CI | not seen | pr-host get_status pending, 0 statuses on `d4f8346` |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
- [std_go_reclaim_context-lease](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/openspec/changes/reclaim-owned-table/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-reclaim-owned-table` from `origin/master`. Stub PR #65. Apply is on HEAD. Next is code review.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Is `New` `New(grace time.Duration)` or `New(Config)` with `Grace` as a field? | bounded asked | assumed — `New(Config)` by value; `Grace` only; copy at `New`; no setter; negative → `DefaultGrace`; zero stays 0. Delete `NewTable`. | explore |
| Is `reclaim/default.go` deleted or left without the singleton? | bounded asked | assumed — delete `reclaim/default.go`. | explore |
| How does `e2e/reclaimprobe` hold the table so two Traefik `New`s still share an incarnation within grace? | bounded asked | assumed — package-level `var` table in `reclaimprobe`, `New(Config{Grace: DefaultGrace})` once; plugin `New` calls `table.Open`. | explore |
| Does `Config` live in `table.go` or a new `config.go`? | additive asked | assumed — `Config` next to `Table` in `reclaim/table.go`. No `reclaim/config.go`. | explore |

## Before merge
- [x] Remove `Default`, package `Open`, `Reset`, and `ResetWith`; construct tables with `New(Config)` and caller-owned lifetime
- [x] Fold spec, usage, README, and `reclaimprobe` off the process table
- [ ] Green CI on HEAD
- [ ] Archive delta into live `std_go_reclaim_context-lease`

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
| Reviewed head | d4f834643d4f8383d2021c48cd24cca6afa40808 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest process table is gone; callers own `New(Config)` with frozen Grace.

Do we have a high-confidence way to reproduce? Yes — `go test -count=1 -timeout 60s ./reclaim/` passed after deleting `default.go`.

Is this the best way to solve the issue? Yes versus dest — remove the singleton rather than add a second config path on `Default`.

### Evidence
What I checked:
- `go test -count=1 -timeout 60s ./reclaim/` passed (worktree, `13c5ab9`)
- Probe package type-checks (`go test` in `e2e/reclaimprobe`)
- `origin/master...HEAD` product: delete `reclaim/default.go`, `New(Config)`, probe/README/usage (`git diff`)
- CI on `d4f8346`: `get_status` pending, 0 statuses (GitHub MCP)

### Rank-up moves
None.
