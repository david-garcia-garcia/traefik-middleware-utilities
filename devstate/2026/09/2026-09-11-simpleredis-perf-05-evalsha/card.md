Developer review: in progress — 2026-09-11T22:25:37Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `SimpleRedis.Eval` keeps `Eval(script, keys, args)` and sends EVALSHA of a client SHA-1 digest, falling back once to EVAL on NOSCRIPT; live catalog `std_go_simpleredis_resp-commands` now requires that path; Yaegi and Pester `/redis` `/dragonfly` prove the miss then hit.

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
Change `simpleredis-evalsha` is archived and folded into `std_go_simpleredis_resp-commands`. CI on 962c586 is still running. 1 item remains.

Priority: P3 — extra EVAL bytes on DestBranch; no wrong answers or outages
Reviewed head: 962c586
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI on the archive commit is still in progress |
| CI proof | 3/6 | Lint succeeded; Test and Integration Tests in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34653892678 |
| Local tests proof | N/A | Remote CI is the proof axis |
| Review resolution | 6/6 | No OPEN PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-simpleredis-perf-05-evalsha pushed | `git` origin 962c586 |
| OpenSpec | simpleredis-evalsha archived | `openspec/changes/archive/2026-09-11-simpleredis-evalsha/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/13 | GitHub PR 13 |
| CI | build 34653892678 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34653892678 | GitHub check runs (Lint success; Test and Integration Tests in progress) |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
- [std_go_simpleredis_resp-commands](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-05-evalsha/openspec/changes/archive/2026-09-11-simpleredis-evalsha/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Archive of `simpleredis-evalsha` on `2026-09-11-simpleredis-perf-05-evalsha` and stub PR 13. Catalog now requires EVALSHA-inside-Eval.

## Explore Decisions
None.

## Before merge
- [x] Land EVALSHA inside `Eval` with NOSCRIPT fallback, Yaegi interp, and live Redis plus Dragonfly proof [P3]
- [x] Fold delta into `std_go_simpleredis_resp-commands` and archive `simpleredis-evalsha`
- [ ] CI on 962c586 (Test and Integration Tests still running) [P3]

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-05-evalsha/devstate/2026/09/2026-09-11-simpleredis-perf-05-evalsha/codereview_standards.md) — 1 total, 0 pending, 0 completed, 1 skipped
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-05-evalsha/devstate/2026/09/2026-09-11-simpleredis-perf-05-evalsha/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-05-evalsha/devstate/2026/09/2026-09-11-simpleredis-perf-05-evalsha/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-05-evalsha/devstate/2026/09/2026-09-11-simpleredis-perf-05-evalsha/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-05-evalsha/devstate/2026/09/2026-09-11-simpleredis-perf-05-evalsha/codereview_performance.md) — 1 total, 0 pending, 0 completed, 1 skipped
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-05-evalsha/devstate/2026/09/2026-09-11-simpleredis-perf-05-evalsha/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-05-evalsha/devstate/2026/09/2026-09-11-simpleredis-perf-05-evalsha/codereview_coverage.md) — 0 total, 0 pending, 0 completed

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 962c586a67eae85d24e90b5810fa4b0f34cbe585 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: keep `Eval(script, keys, args)` and send EVALSHA with a NOSCRIPT→EVAL fallback so callers and scripts (KEYS, no `table.maxn`) stay the same.

Do we have a high-confidence way to reproduce? Yes — compiled fake argv, Yaegi `EvalNoScript`, and Pester SCRIPT FLUSH/EXISTS on `/redis` and `/dragonfly`.

Is this the best way to solve the issue? Yes — digest plus fallback beats shipping the body, and avoids `SCRIPT LOAD` at Init.

### Evidence
What I checked:
- `origin/master...HEAD` at 962c586 (Eval EVALSHA + NOSCRIPT fallback; catalog `std_go_simpleredis_resp-commands` folded; change archived)
- FindSpecHost: fold `std_go_simpleredis_resp-commands` high; validate_spec_map and validate_artifact_names clean
- Seven-axis files under `devstate/2026/09/2026-09-11-simpleredis-perf-05-evalsha/`; 0 hard/missing/wrong remaining
- PR 13 OPEN; CI run 34653892678 in progress (Lint success)
- qualify: qualified-with-gaps; localTests: passed; change: simpleredis-evalsha (`handoff.yaml`)

### Rank-up moves
- Probe SHA-1 copy vs `scriptSHA1Hex` (Standards judgement, skipped)
- Digest map has no cap (Performance judgement, skipped; callers pass consts)
