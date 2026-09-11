Developer review: in progress — 2026-09-11T21:41:39.941Z

## What this changes
**Operators.** CI `test` job now sets `LEAKYBUCKET_LIVE_REDIS` / `LEAKYBUCKET_LIVE_DRAGONFLY` on the existing Redis `:6379` and Dragonfly `:6380` services, and `go test` timeout is 5m.

**Admin users.** None.

**Developers.** New `leakybucket/` package: `NewMemory` / `NewRedis`, `Add`/`Take`/`Level`, Redis EVAL leak-then-add HASH `{water,last}`, Kong-style `sync_rate` flush, reclaim hooks on Redis only. Memory map capped at 65536 like `tokenbucket`.

**End users.** None.

## Motivation
Middleware authors who need a fill clock (pour water, constant leak, overflow = deny) have no package on `master`. Dest already ships `tokenbucket/` (Traefik refill + burst) and `windowcounter/` (Kong sliding hits + `INCRBY` flush) and tells callers not to mix those clocks.

If we do not add `leakybucket/`, a plugin either inverts Traefik’s token Lua and calls it leaky, or `SET`s a `{water, last}` snapshot from each replica (last-write-wins / double-leak), or reuses window integers for a job that is not a window.

```mermaid
flowchart LR
  need[Need fill plus leak]
  dest[Dest has tokenbucket and windowcounter]
  wrong[Inverted Lua or SET blob or window math]
  need --> dest --> wrong
```

## Merge readiness
Seven-axis review applied; usage packet Key files and memory-cap Gotcha caught up; CI on this head is still running; OpenSpec change is not archived yet. 1 item remains.

Priority: P3 — new library; no current operator or end-user harm

Reviewed head: f8e92b4
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3 | CI on this head is still in progress |
| CI proof | 3 | Lint, Test, Integration in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34650569484 |
| Local tests proof | N/A | Remote PR; CI covers it |
| Review resolution | 6 | No open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-leaky-bucket pushed | git / origin |
| OpenSpec | add-leakybucket | openspec/changes/add-leakybucket/ |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/9 | pr-host |
| CI | build 34650569484 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34650569484 | GitHub MCP get_check_runs |
| Local tests | passed | handoff.yaml localTests; `go test -short ./leakybucket/` after review fixes |
| PR comments | no comments | comments: none |

## Specs
- [std_go_leakybucket_pour](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/openspec/changes/add-leakybucket/proposal.md) — added
- [std_go_leakybucket_sync-flush](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/openspec/changes/add-leakybucket/proposal.md) — added

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-11-leaky-bucket` → PR #9 → usage packet caught up; archive next.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| How is `{water, last}` encoded in Redis so EVAL can leak once then add a delta without a replica SET? | additive asked | assumed — HASH fields `water` (float string) and `last` (unix microseconds) on one KEYS[1]; EVAL HGETALL / leak / add / HSET / EXPIRE; Go never writes the hash except through that script. | explore |
| Is time-until-not-full a required return on Add/Take or omitted? | additive asked | assumed — always return it as the third value (duration until there is room for one more unit; 0 if already not full). Level does not return it. | explore |
| Are Add and Take aliases, or is Take n=1 only? | additive asked | assumed — `Add(key, n int64)` pours n (`n < 1` errors); `Take(key)` is Add(key, 1). | explore |
| What units make in-memory and Redis agree on admit/deny for the same leak/capacity sequence? | additive asked | assumed — leak float64 water/second, capacity float64, n int64, elapsed unix microseconds, Lua `tonumber` on the same ARGV as Go. | explore |
| What live env var names, and does CI reuse the existing Redis/Dragonfly services? | additive asked | assumed — `LEAKYBUCKET_LIVE_REDIS` and `LEAKYBUCKET_LIVE_DRAGONFLY`; CI sets them to `127.0.0.1:6379` and `127.0.0.1:6380`; skip under `-short` or unset locally; CI must not skip. | explore |
| Does the in-memory store expose Sleep/Wake/Close? | additive asked | assumed — only the Redis limiter (the type that may start a ticker) exports Sleep/Wake/Close; memory has no ticker. | explore |
| Where does Redis EXPIRE ttl come from? | additive asked | assumed — constructor `ttl` on both stores, min 1s; script EXPIRE that many seconds; missing hash is empty water. | explore |

## Before merge
- [x] Stub PR #9 open
- [x] Explore recorded
- [x] OpenSpec `add-leakybucket` proposed
- [x] Land `leakybucket/` with unit, live Redis/Dragonfly, and Yaegi proof
- [x] Apply seven-axis hard findings
- [x] Usage packet Key files and memory-cap Gotcha
- [ ] Archive specs and drop WIP from the PR title

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/devstate/2026/09/2026-09-11-leaky-bucket/codereview_standards.md) — 1 total, 0 pending, 0 completed, 1 skipped
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/devstate/2026/09/2026-09-11-leaky-bucket/codereview_nitpicks.md) — 3 total, 0 pending, 3 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/devstate/2026/09/2026-09-11-leaky-bucket/codereview_spec.md) — 1 total, 0 pending, 1 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/devstate/2026/09/2026-09-11-leaky-bucket/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/devstate/2026/09/2026-09-11-leaky-bucket/codereview_performance.md) — 3 total, 0 pending, 2 completed, 1 skipped
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/devstate/2026/09/2026-09-11-leaky-bucket/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/devstate/2026/09/2026-09-11-leaky-bucket/codereview_coverage.md) — 4 total, 0 pending, 2 completed, 2 skipped

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 2 added / 0 modified | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | f8e92b420e7fdc66e75f8cadafeb1cfd1548c1dd | Card must match the branch you measured |

### Stored data model
- New: Redis HASH key (caller-prefixed opaque key) with fields `water` and `last`.

### Technical review
Best possible solution: Dest already owns token-bucket refill and Kong window+flush as separate packages; a third package for the water clock matches that split.

Do we have a high-confidence way to reproduce? Yes — dest had no `leakybucket/`; review hard findings (memory cap, Yaegi deny-at-cap, Redis `n < 1`, EVAL until parse) are in `c8855c3`.

Is this the best way to solve the issue? Yes — I.371 meter + Kong delta timer, not inverted Traefik Lua.

### Evidence
What I checked:
- `go test -short ./leakybucket/` passed after review fixes
- GitHub MCP get_check_runs: build 34650569484 Lint/Test/Integration in_progress
- `devdocs-impact.md`: stale-usage Key files + Gotcha produced in `f8e92b4`

### Rank-up moves
- Extract shared EVAL ARGV helper (Standards judgement skip)
- Pipeline flush when SimpleRedis grows a pipeline (Performance skip)
