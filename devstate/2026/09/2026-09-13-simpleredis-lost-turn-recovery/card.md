Developer review: ready for review — 2026-09-13T09:39:20Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** SimpleRedis restores missing in-use turns at pool-wait when idle is empty and no still-running command holds a socket, exposes read-only `LostTurns()`, and does not defer `release`. Spec `std_go_simpleredis_tcp-session` treats wait-not-dial as backpressure only when sockets are actually owned.

**End users.** None.

## Motivation
SimpleRedis returns an in-use-turn only from `release` after `do`. On DestBranch that call is not deferred (PR 29 discarded defer-release). A panic between `borrow` and `release` drops one turn forever. There is no reaper, and `OverFrees()` only counts extra returns.

Measured on DestBranch with `PoolSize` 2: one warm socket, then two recovered panics after `borrow`. `inUseTurns` ended 0/2, idle 0, TCP accepts 2, next `Get` returned `redis:unreachable` while Redis was healthy. Traefik recovers a panicking middleware per request, so the process stays up and quietly loses a turn each time. After `PoolSize` recovered panics the client is dead for the process lifetime.

If this does not merge, `PoolSize` recovered panics convert a live client into a permanent outage that looks like a Redis failure. No reachable panic was found in compiled Go (`readReply` fuzz 7.04M execs / 91s), so this is a latent hazard with unbounded blast radius, not a live crash. Yaegi panics are already documented in this package.

## Merge readiness
CI on PR 66 succeeded. 0 items remain.

Priority: P2 — latent operator outage with a restart workaround, not a live compiled crash today
Reviewed head: 0e9e8c2
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | OPEN PR, CI succeeded, no open review comments |
| CI proof | 6/6 | build 34749762357 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34749762357 |
| Local tests proof | N/A | `localTests: passed`; remote CI is the proof axis |
| Review resolution | 6/6 | OPEN PR 66, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-lost-turn-recovery pushed | `git` HEAD 0e9e8c2 |
| OpenSpec | simpleredis-lost-turn-recovery | archived |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/66 | GitHub |
| CI | build 34749762357 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34749762357 | GitHub check runs |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | comments: none |

## Specs
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-lost-turn-recovery/openspec/changes/archive/2026-09-13-simpleredis-lost-turn-recovery/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket → branch `2026-09-13-simpleredis-lost-turn-recovery` → PR 66 → CI 34749762357 green.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Refill missing turns at errPoolWait when owned live sockets are zero, or make the turn a leased resource with a reaper? | additive asked | assumed — refill at errPoolWait when idle is empty and heldSockets is 0; no lease object and no goroutine reaper | explore |
| Where is the dialed-but-not-yet-released counter incremented so a recovered panic looks like zero owned sockets and a busy Get does not? | additive asked | assumed — heldSockets atomic, increment in exec after borrow with defer decrement, and around dial inside borrow. Direct borrow then panic never enters exec so refill is correct | explore |
| Does the nanosecond window after taking a turn and before heldSockets increments let recovery fire and dial past PoolSize? | additive incidental | assumed — do not add a mutex or reaper for that window; hung TCP is covered by the dial increment | explore |
| Should this change also close TCP fds left behind when borrow returns a conn that is never released? | additive incidental | assumed — do not track or close those fds; refill lets the next command dial | explore |

## Before merge
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-lost-turn-recovery/devstate/2026/09/2026-09-13-simpleredis-lost-turn-recovery/codereview_standards.md) — 2 total, 0 pending, 2 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-lost-turn-recovery/devstate/2026/09/2026-09-13-simpleredis-lost-turn-recovery/codereview_nitpicks.md) — 1 total, 0 pending, 1 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-lost-turn-recovery/devstate/2026/09/2026-09-13-simpleredis-lost-turn-recovery/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-lost-turn-recovery/devstate/2026/09/2026-09-13-simpleredis-lost-turn-recovery/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-lost-turn-recovery/devstate/2026/09/2026-09-13-simpleredis-lost-turn-recovery/codereview_performance.md) — 1 total, 0 pending, 1 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-lost-turn-recovery/devstate/2026/09/2026-09-13-simpleredis-lost-turn-recovery/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-lost-turn-recovery/devstate/2026/09/2026-09-13-simpleredis-lost-turn-recovery/codereview_coverage.md) — 3 total, 0 pending, 2 completed, 1 skipped

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 0e9e8c2f951add47582bfac231215f4eb982af34 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Restore leaked turns at pool-wait when the client owns zero sockets, instead of defer-release (PR 29 discarded that).

Do we have a high-confidence way to reproduce? Yes, TestBugLostInUseTurnBricksPoolPermanently failed on DestBranch (turns=0/2, redis:unreachable) and passes after the refill.

Is this the best way to solve the issue? Yes versus DestBranch: it fixes permanence without reversing PR 29, and recovery does not fire while sockets are genuinely busy.

### Evidence
What I checked:
- Default suite `go test -count=1 -timeout 300s ./simpleredis/` passed
- Pool tests `-count=5` passed
- `go vet ./simpleredis/` clean
- CI 34749762357: Unit, Unit race, Lint, Go E2E Redis, Go E2E Dragonfly, Integration Tests, Integration Tests Redis, Integration Tests Dragonfly all succeeded
- PR 29 owner: discarded defer-release as too much complexity for Yaegi recovering the request while the turn stays lost

### Rank-up moves
- TCP fds left behind when borrow returns a conn that is never released stay leaked, as on DestBranch. Refill dials a new socket.
