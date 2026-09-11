Developer review: in progress — 2026-09-11T21:21:10Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Prepare bus only. Dest `SimpleRedis.Eval` still sends `EVAL` plus the full Lua body on every call.

**End users.** None.

## Motivation
`Eval` is the hot-path verb for Redis and Dragonfly rate-limit decisions (token-bucket Allow, window-counter flush). On DestBranch every call copies the script onto the wire. The Traefik token-bucket script is ~470 bytes; at limiter traffic that is redundant Lua Redis already has compiled.

If this PR does not land, every exact-mode hit keeps paying that extra ~500 bytes and Redis-side hash of a body it already knows. Limiters still admit correctly. The miss is wire and parse cost, not a wrong allow/deny.

```mermaid
sequenceDiagram
  participant Caller
  participant Engine as Redis or Dragonfly
  Caller->>Engine: EVAL plus full Lua body
  Note over Caller,Engine: DestBranch: every limiter decision
  Engine-->>Caller: script result
```

## Merge readiness
Prepare is recorded; product Eval is unchanged. 1 item remains before merge (the apply).

Priority: P3 — extra EVAL bytes on DestBranch; no wrong answers or outages
Reviewed head: bf90455
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI still queued; product apply has not started |
| CI proof | 3/6 | Checks queued on run 34648936900 |
| Local tests proof | N/A | Before implement; remote CI is the proof axis |
| Review resolution | 6/6 | No OPEN PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-simpleredis-perf-05-evalsha pushed | `git` origin |
| OpenSpec | none | `openspec/` unchanged vs master |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/13 | GitHub PR 13 |
| CI | build 34648936900 queued https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34648936900 | GitHub checks |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local perf-05 finding is on branch `2026-09-11-simpleredis-perf-05-evalsha` and stub PR 13. Explore is next; `Eval` is not changed yet.

## Explore Decisions
None.

## Before merge
- [ ] Land EVALSHA inside `Eval` with NOSCRIPT fallback, Yaegi interp, and live Redis plus Dragonfly proof [P3]
- [x] Stub review PR opened

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | bf90455451afc5f3c9f01c2d16dbaa94cf50c0ae | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: keep `Eval(script, keys, args)` and send EVALSHA with a NOSCRIPT→EVAL fallback, as go-redis `Script.Run` does, so callers and scripts (KEYS, no `table.maxn`) stay the same.

Do we have a high-confidence way to reproduce? Yes, dest `Eval` always appends `EVAL` plus the script (`simpleredis/simpleredis.go`); fake and Pester `/redis` `/dragonfly` exist to extend.

Is this the best way to solve the issue? Yes — digest plus fallback beats shipping the body or `unsafe` zero-copy, and avoids `SCRIPT LOAD` at Init.

### Evidence
What I checked:
- `git ls-tree origin/master simpleredis` present (7dc4b05)
- `Eval` / `replyError` / fake EVAL-only (`simpleredis/simpleredis.go`, `simpleredis_test.go`)
- Spec forbids EVALSHA (`openspec/specs/std_go_simpleredis_resp-commands/spec.md`)
- Research: `ext_redis_eval`, `ext_dragonfly_eval`, go-redis `script.go` extract
- PR 13 OPEN, checks queued (run 34648936900)
- qualify: qualified-with-gaps (`handoff.yaml`)

### Rank-up moves
- Dedicated `knowledge/research/` packet for EVALSHA/NOSCRIPT wire text on Redis 7 and Dragonfly v1.40.2 (explore/research).
