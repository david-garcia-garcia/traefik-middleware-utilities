Developer review: in progress — 2026-09-11T21:26:38.595Z

## What this changes
**Operators.** CI `test` job now sets `LEAKYBUCKET_LIVE_REDIS` / `LEAKYBUCKET_LIVE_DRAGONFLY` on the existing Redis `:6379` and Dragonfly `:6380` services, and `go test` timeout is 5m.

**Admin users.** None.

**Developers.** New `leakybucket/` package: `NewMemory` / `NewRedis`, `Add`/`Take`/`Level`, Redis EVAL leak-then-add HASH `{water,last}`, Kong-style `sync_rate` flush, reclaim hooks on Redis only.

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
Library is on the branch; CI succeeded; OpenSpec change is not archived yet. 1 item remains.

Priority: P3 — new library; no current operator or end-user harm

Reviewed head: 6ba0b36
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6 | CI succeeded; checklist still has archive/PR |
| CI proof | 6 | Lint, Test, Integration succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34649263030 |
| Local tests proof | N/A | Remote PR; CI covers it |
| Review resolution | 6 | No open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-leaky-bucket pushed | git / origin |
| OpenSpec | add-leakybucket | openspec/changes/add-leakybucket/ |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/9 | pr-host |
| CI | build 34649263030 success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34649263030 | GitHub MCP get_check_runs |
| Local tests | passed | handoff.yaml localTests; `go test ./leakybucket/...` and Go 1.21 docker `./...` with live engines |
| PR comments | no comments | comments: none |

## Specs
- [std_go_leakybucket_pour](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/openspec/changes/add-leakybucket/proposal.md) — added
- [std_go_leakybucket_sync-flush](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-leaky-bucket/openspec/changes/add-leakybucket/proposal.md) — added

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-11-leaky-bucket` → PR #9 → code review next.

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
- [ ] Archive specs and drop WIP from the PR title

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 2 added / 0 modified | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 6ba0b36acd3cf73216e809ba3f60d6ff6c5eaac9 | Card must match the branch you measured |

### Stored data model
- New: Redis HASH key (caller-prefixed opaque key) with fields `water` and `last`.

### Technical review
Best possible solution: Dest already owns token-bucket refill and Kong window+flush as separate packages; a third package for the water clock matches that split.

Do we have a high-confidence way to reproduce? Yes — dest had no `leakybucket/`; CI Test on this head succeeded after prefixing live keys and raising the suite timeout.

Is this the best way to solve the issue? Yes — I.371 meter + Kong delta timer, not inverted Traefik Lua.

### Evidence
What I checked:
- `go test ./leakybucket/...` passed locally (live Redis/Dragonfly + Yaegi)
- Go 1.21 docker `go test ./...` with all live env vars passed after `leakybucket:` key prefix
- CI run 34649263030 Lint/Test/Integration success

### Rank-up moves
None.
