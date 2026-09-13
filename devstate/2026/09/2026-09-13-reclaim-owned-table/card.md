Developer review: in progress — 2026-09-13T08:31:00Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `reclaim` no longer has `Default` / package `Open` / `Reset` / `ResetWith`. Callers create a table with `New(Config)` (Grace copied and frozen). `e2e/reclaimprobe` holds a package-level table. README and `std_go_reclaim` / `std_go_backendbackoff` usage follow. Canceled-bind tests from dest construct with `New(Config)`.

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
Archive folded the delta into live `std_go_reclaim_context-lease`. CI on HEAD is still running.

Priority: P3 — spec and internal API clarity, no current operator or end-user harm
Reviewed head: 221ce0f
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI on this SHA is in progress |
| CI proof | 3/6 | 8 checks in progress on `221ce0f` |
| Local tests proof | N/A | `prHost` is github; CI covers remote |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-owned-table pushed | `git push` `221ce0f` |
| OpenSpec | reclaim-owned-table | `openspec/changes/archive/2026-09-13-reclaim-owned-table/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/65 | pr-host List |
| CI | build 34747857540 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34747857540 | pr-host get_check_runs |
| Local tests | passed | handoff.yaml localTests (`go test -short -timeout 2m -count=1 ./...`) |
| PR comments | no comments | get_comments empty, review threads 0 |

## Specs
- [std_go_reclaim_context-lease](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/openspec/changes/archive/2026-09-13-reclaim-owned-table/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-reclaim-owned-table` from `origin/master`. PR #65. Delta archived into the live context-lease spec. Waiting on CI for pullrequest.

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
- [x] Archive delta into live `std_go_reclaim_context-lease`
- [ ] Green CI on HEAD

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/devstate/2026/09/2026-09-13-reclaim-owned-table/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/devstate/2026/09/2026-09-13-reclaim-owned-table/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/devstate/2026/09/2026-09-13-reclaim-owned-table/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/devstate/2026/09/2026-09-13-reclaim-owned-table/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/devstate/2026/09/2026-09-13-reclaim-owned-table/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/devstate/2026/09/2026-09-13-reclaim-owned-table/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-owned-table/devstate/2026/09/2026-09-13-reclaim-owned-table/codereview_coverage.md) — 0 total, 0 pending, 0 completed

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 221ce0ffe12f4905e163c8fb007f0ecf6f7bc81d | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest process table is gone; callers own `New(Config)` with frozen Grace.

Do we have a high-confidence way to reproduce? Yes — `go test -short -timeout 2m -count=1 ./...` passed after dest merge of canceled-bind tests onto `New(Config)`.

Is this the best way to solve the issue? Yes versus dest — remove the singleton rather than add a second config path on `Default`.

### Evidence
What I checked:
- `go test -count=1 -timeout 60s ./reclaim/` passed (worktree, `221ce0f`)
- `go test -short -timeout 2m -count=1 ./...` passed (worktree, `221ce0f`)
- `origin/master...HEAD` product: delete `reclaim/default.go`, `New(Config)`, probe/README/usage, canceled-bind tests call `New` (`git diff`)
- CI on `221ce0f`: 8 checks in progress, run 34747857540 (GitHub MCP get_check_runs)

### Rank-up moves
None.
