Developer review: in progress — 2026-09-11T19:05:01.275Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `knowledge/research/ext_traefik_ratelimiter_token-bucket/` records Traefik RateLimit token-bucket math (Lua + in-memory). `tokenbucket/` is not on dest and is not implemented yet.

**End users.** None.

## Motivation
Middleware authors who need Traefik RateLimit behaviour (refill `rate`, cap `burst`, wait, refund when wait exceeds maxDelay) have no package on `master`. Dest already ships `windowcounter/` for Kong sliding windows and says a token bucket would be a separate package.

If we do not add that package, a plugin either imports the window counter (wrong clock) or copies Traefik’s Lua with `table.maxn` and `go-redis`, which Yaegi and Dragonfly will not run.

```mermaid
flowchart LR
  need[Need refill plus burst]
  dest[Dest has windowcounter only]
  wrong[Wrong clock or copied Traefik Lua]
  need --> dest --> wrong
```

## Merge readiness
Prepare grounded; explore has not run. 3 items remain.

Priority: P3 — missing library and spec; no current operator or end-user harm

Reviewed head: 5f6907e
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1 | New commits on the stub PR are not measured by CI yet |
| CI proof | 1 | Stub commit checks succeeded; HEAD after research/prepare not seen |
| Local tests proof | N/A | Before implement |
| Review resolution | 6 | No open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-traefik-token-limiter pushed | git / origin |
| OpenSpec | none | handoff.yaml |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/8 | pr-host |
| CI | not seen | new HEAD after prepare files; prior stub run 34636391855 succeeded |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | comments: none |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-11-traefik-token-limiter` → stub PR #8 → prepare `qualified-with-gaps`. Explore is next.

## Explore Decisions
None.

## Before merge
- [x] Stub PR #8 open
- [x] Traefik token-bucket research notes
- [ ] Explore `tokenbucket/` Allow mapping, clock units, live env
- [ ] Propose OpenSpec change
- [ ] Implement library + live Redis/Dragonfly + Yaegi tests

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | No spec.md in dest...HEAD |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 5f6907e8 pending bus commit | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: not implemented; dest still has no token-bucket package.

Do we have a high-confidence way to reproduce? Yes — dest tree has `windowcounter/` and no `tokenbucket/`; Traefik Lua still uses `table.maxn`.

Is this the best way to solve the issue? Yes — separate package as dest README already reserved; do not fold into `windowcounter/`.

### Evidence
What I checked:
- dest `origin/master` 705597a (`windowcounter/`, README token-bucket reservation)
- Traefik `lua.go` / `in_memory_limiter.go` / `redis_limiter.go` @903e8a9
- GitHub MCP `get_me`, PR #8, check runs on stub commit
- `simpleredis.Eval` present

### Rank-up moves
None.
