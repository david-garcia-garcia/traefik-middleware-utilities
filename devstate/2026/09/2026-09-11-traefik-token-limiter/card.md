Developer review: in progress — 2026-09-11T19:10:19.055Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `knowledge/research/ext_traefik_ratelimiter_token-bucket/` records Traefik RateLimit token-bucket math. Explore locked `tokenbucket/` Allow mapping, Lua clock units, and `TOKENBUCKET_LIVE_*` env names. The package is not on dest yet.

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
Explore recorded; propose has not run. 3 items remain.

Priority: P3 — missing library and spec; no current operator or end-user harm

Reviewed head: 0a301de
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1 | HEAD after explore is not measured by CI yet |
| CI proof | 1 | pushed; check runs not seen on 0a301de |
| Local tests proof | N/A | Before implement |
| Review resolution | 6 | No open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-traefik-token-limiter pushed | git / origin |
| OpenSpec | none | handoff.yaml |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/8 | pr-host |
| CI | not seen | GitHub MCP get_check_runs total_count 0 |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | comments: none |

## Specs
None.

## Deviations from the ask
- taken: README leaky-bucket row replacement → add a Token bucket row beside Window counter — `README.md` — dest already replaced leaky-`bucket/` with `windowcounter/`. Requester: not asked.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-11-traefik-token-limiter` → stub PR #8 → explore locked Allow/clock/live-env. Propose is next.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| How does Allow's allowed bool map Traefik Lua (always "true") plus HTTP 429 when wait > maxDelay or delay is nil? | additive asked | assumed — Lua still returns "true"; Go maps allowed=false wait=0 when no burst; allowed=false wait>maxDelay after refund; allowed=true wait>=0 when admitted now or after waiting ≤ maxDelay. Library never sleeps. | explore |
| Microsecond Lua clock vs nanosecond x/time/rate — which unit so both stores agree? | additive asked | assumed — public API is time.Duration + reqs/s; both stores convert like Traefik Redis (rate/1e6, UnixMicro, maxDelay.Microseconds()); in-memory uses those formulas, not x/time/rate. | explore |
| Live env var names, and does CI reuse the existing Redis/Dragonfly pair? | additive asked | assumed — TOKENBUCKET_LIVE_REDIS / TOKENBUCKET_LIVE_DRAGONFLY; reuse CI 127.0.0.1:6379 and :6380; skip without addrs or under -short; CI sets both. | explore |
| Do in-memory buckets need reclaim Sleep/Wake/Close? | additive incidental | assumed — no reclaim hooks on the limiter in v1; memory expires entries on Allow using caller ttl; no per-key ticker. | explore |
| Default ttl if the caller does not pass Traefik’s formula? | additive asked | assumed — no default; ttl < 1s fails New; document Traefik’s formula in usage (2s when rate ≥ 1, else 1 + int(1/rate)). | explore |
| What happens when rate <= 0 (Traefik average: 0 / rate.Inf)? | additive incidental | assumed — New errors when rate <= 0 or burst < 1 or maxDelay < 0; do not copy Inf short-circuit. | explore |

## Before merge
- [x] Stub PR #8 open
- [x] Explore Allow mapping, clock units, live env
- [ ] Propose OpenSpec change
- [ ] Implement `tokenbucket/` + live Redis/Dragonfly + Yaegi tests

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
| Reviewed head | 0a301deeffd31f178c2ac8114c1570b6102c478e | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: not implemented; dest still has no token-bucket package. Explore chose Lua-formula memory plus `simpleredis.Eval` so both stores share one meaning.

Do we have a high-confidence way to reproduce? Yes — dest tree has `windowcounter/` and no `tokenbucket/`; Traefik Lua still uses `table.maxn`.

Is this the best way to solve the issue? Yes — separate package as dest README already reserved; do not fold into `windowcounter/`.

### Evidence
What I checked:
- dest `tokenbucket/` not found; README line 25 reserves a separate package
- `simpleredis.Eval` present (`simpleredis.go:154`)
- Traefik research pin 903e8a9 (Lua always-true, Inf 429, EVALSHA)
- GitHub MCP PR #8 comments empty; check_runs total_count 0 on HEAD 0a301de

### Rank-up moves
None.
