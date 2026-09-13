Developer review: in progress — 2026-09-13T15:14:55Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `exec` defers `release` via `runOnConn`. `borrow` defers `freeInUseTurn` unless the socket is handed off. A recovered panic returns the in-use turn and closes the socket. Permanent proofs: `simpleredis/panic_safety_test.go`, `simpleredis/yaegi_defer_test.go`. Spec `std_go_simpleredis_tcp-session` now requires that guarantee.

**End users.** None.

## Motivation
SimpleRedis spends one in-use turn for every borrowed socket. Dest fills that turn channel once in `New` and never puts a lost turn back. `exec` calls `release` only after `do` returns. A panic between those two calls keeps the turn and the socket fd.

On dest, Traefik recovers a panicking middleware per request, so the process stays up. After `PoolSize` recovered panics the channel is empty, idle is empty, and every later command is `redis:unreachable` against a healthy Redis with zero open sockets. Operators see an outage. PR #29 closed on the belief that a deferred `release` would not run under Yaegi. A probe on Yaegi v0.16.1 ran the deferred restore for an explicit interpreted panic, the interpreter `errors.As` panic, and a nil-map write. Until this lands, one panicking plugin request per live slot permanently bricks the client.

```mermaid
sequenceDiagram
    participant Traefik
    participant Exec
    participant Pool
    Traefik->>Exec: request
    Exec->>Pool: borrow
    Note over Exec: dest release is not deferred
    Exec->>Exec: panic inside do
    Traefik->>Traefik: recover the request
    Note over Pool: turn and fd stay lost
    Note over Traefik: after PoolSize panics every command is unreachable
```

## Merge readiness
Product, spec archive, and usage gotcha are on the branch. CI on the latest head is not measured yet. 1 item remains.

Priority: P1 — dest is permanently unreachable after PoolSize recovered panics
Reviewed head: 79cb57a
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Latest head CI not seen |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | remote PR; localTests passed |
| Review resolution | 6/6 | OPEN PR, no comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-panic-safe-release pushed | `git` HEAD 79cb57a |
| OpenSpec | simpleredis-panic-safe-release | archived |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/71 | GitHub |
| CI | not seen | GitHub check-run adapter empty for current head |
| Local tests | passed | `go vet ./simpleredis/` clean; `go test -count=1 -timeout 300s ./simpleredis/` 14.950s; `-count=5 -run "Pool|Panic|Release|Borrow|Turn"` 7.922s; no pre-existing test files modified |
| PR comments | no comments | comments: none |

## Specs
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-panic-safe-release/openspec/changes/archive/2026-09-13-simpleredis-panic-safe-release/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-13-simpleredis-panic-safe-release` → PR 71 → archived OpenSpec change → product HEAD 86dda81 → spec archive 79cb57a.

## Explore Decisions
None.

## Before merge
- [ ] CI on latest head
- [x] [P1] Land `runOnConn` deferred release
- [x] [P1] Land `borrow` `handedOff`
- [x] Permanent panic-in-`do` test and Yaegi defer-on-panic probe
- [x] Update `resp.go` comment, tcp-session spec, and `std_go_simpleredis.md` gotcha

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-panic-safe-release/devstate/2026/09/2026-09-13-simpleredis-panic-safe-release/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-panic-safe-release/devstate/2026/09/2026-09-13-simpleredis-panic-safe-release/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-panic-safe-release/devstate/2026/09/2026-09-13-simpleredis-panic-safe-release/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-panic-safe-release/devstate/2026/09/2026-09-13-simpleredis-panic-safe-release/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-panic-safe-release/devstate/2026/09/2026-09-13-simpleredis-panic-safe-release/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-panic-safe-release/devstate/2026/09/2026-09-13-simpleredis-panic-safe-release/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-panic-safe-release/devstate/2026/09/2026-09-13-simpleredis-panic-safe-release/codereview_coverage.md) — 1 total, 0 pending, 0 completed, 1 skipped

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 79cb57a4438b694ce2dd353e83854586f14b1400 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest now defers `release` in `runOnConn` and `freeInUseTurn` in `borrow` unless handed off. Same design as `origin/proto-defer-net`, applied onto current dest.

Do we have a high-confidence way to reproduce? Yes. Dest throwaway: `turns=0/2` then `redis:unreachable`. After apply: `TestPanicInDoReturnsTurnAndClosesSocket` `turns=2/2 idle=0 accepts=3 OverFrees=0`.

Is this the best way to solve the issue? Yes versus dest: prevent the leak. Closed #66 / #70 refilled turns without closing the panicked fd.

### Evidence
What I checked:
- `go vet ./simpleredis/` clean
- `go test -count=1 -timeout 300s ./simpleredis/` passed in 14.950s
- `go test -count=5 -timeout 600s -run "Pool|Panic|Release|Borrow|Turn" ./simpleredis/` passed in 7.922s
- no pre-existing `simpleredis/*_test.go` edited
- `TestYaegi_DeferRunsOnPanic` three panic classes, turns=0
- Yaegi pin resolved: this module and Traefik v3.7.11 require v0.16.1
- Seven-axis review: six clean; coverage 1 skipped (no dest injection for panic inside `borrow` without a product hook)
- `validate_artifact_names` OK; `validate_spec_map` OK

### Rank-up moves
None.
