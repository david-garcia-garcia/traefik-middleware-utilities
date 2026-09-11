Developer review: ready for review — 2026-09-11T22:00:03Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** SimpleRedis now proves idle ≤ 8 after more than eight overlapping commands (hold fake, excess sockets closed, not leaked), in-flight Close closes the socket on release, and borrow's second closed check via `afterIdleScanForTest`; Pester fires 16 parallel `/redis` and `/dragonfly` requests then bare `CLIENT LIST` remaining ≤ 8; OpenSpec change `test-idle-cap-and-release-after-close` rewrites `std_go_simpleredis_tcp-session` to those contracts.

**End users.** None.

## Motivation
SimpleRedis already closes a socket when the idle list is full or the client is Closed. On master, no test executes those release branches. The concurrent-pool test starts eight goroutines and asserts at most eight TCP accepts, which eight goroutines cannot exceed. The Close test calls Get after Close, which returns `redis:unreachable` from borrow's first closed check, so release never runs. Live Pester `/redis` and `/dragonfly` each send one request, so they cannot prove overlapped commands leave at most eight idle sockets on either engine.

If those guards regress, idle sockets grow without bound in a long-lived Traefik process, or Close on config reload pools sockets into a client that is supposed to be dead. This ticket is missing proof, not a demonstrated production leak today.

```mermaid
sequenceDiagram
  participant Test
  participant Borrow
  participant Release
  Note over Test,Release: master Close test
  Test->>Borrow: Get after Close
  Borrow-->>Test: unreachable
  Note over Release: release never called
  Note over Test,Release: intended proof
  Test->>Borrow: command in flight
  Test->>Test: Close
  Borrow->>Release: command finishes
  Release-->>Test: socket closed, idle empty
```

## Merge readiness
Apply is on the branch. Unit idle-cap and Close tests, live Redis/Dragonfly overlap, and CI succeeded. 0 items remain.

Priority: P3 — tests and proof of existing idle-cap and Close-on-release guards; no current user or operator harm
Reviewed head: c9824ee
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | CI succeeded; no open PR comments |
| CI proof | 6/6 | succeeded — [run 34651786131](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34651786131) |
| Local tests proof | N/A | prHost github; CI proof covers remote |
| Review resolution | 6/6 | no PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-simpleredis-test-02-idle-cap pushed | `git push` to origin; HEAD c9824ee |
| OpenSpec | test-idle-cap-and-release-after-close | `openspec/changes/test-idle-cap-and-release-after-close/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/14 | GitHub PR 14 |
| CI | Lint success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34651786131/job/103435375204; Test success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34651786131/job/103435375048; Integration Tests success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34651786131/job/103435374839 | GitHub check runs on PR 14 |
| Local tests | passed | handoff.yaml localTests; `go test ./...` and `./Test-Integration.ps1` 10/10 |
| PR comments | no comments | comments.md absent; PR comment list empty |

## Specs
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-test-02-idle-cap/openspec/changes/test-idle-cap-and-release-after-close/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Idle-cap and Close-on-release proof is on branch `2026-09-11-simpleredis-test-02-idle-cap`, PR 14, CI run 34651786131 succeeded (Lint, Test, Integration Tests). Next phase is code review of the apply.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| How to drive genuine overlap on DestBranch without `startSlowRedis` / `bench_test.go`? | additive asked | assumed — add a same-file hold fake; 16 Gets then `len(idle) <= 8` and live sockets equal idle. Do not add `bench_test.go`. | propose |
| How does Pester observe idle socket count vs cap on live Redis and Dragonfly? | additive asked | assumed — 16 parallel `/redis` and `/dragonfly` requests, then bare `CLIENT LIST` via `docker compose exec redis redis-cli` (and `-h dragonfly`). Remaining clients ≤ 8. No idle getter. | propose |
| Can `Close` between idle scan and `dial` be made deterministic without a test hook? | additive asked | assumed — nil-checked same-package hook after idle-scan unlock; test sets it to `Close`. Do not leave `:236-238` untested. | propose |
| Does the spec SHALL "Concurrent commands SHALL not open more than eight connections" stay, given usage says in-flight dials are not capped? | bounded asked | assumed — rewrite SHALL/scenario to idle ≤ 8 after overlap of more than eight; in-flight MAY exceed eight. Repair the 8-goroutine test. Add in-flight-Close scenario. | propose |

## Before merge
- [x] Unit-test idle cap and in-flight Close so those `release` branches can fail
- [x] Live overlap on Redis and Dragonfly must not leak idle sockets beyond the cap
- [x] CI succeeded on this PR
- [x] Stub PR opened
- [x] Propose artifacts written

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | c9824eee7780caad1a8c60632ea65c54ebbcb629 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: DestBranch already caps idle at eight and closes in-flight sockets after Close; this change makes those branches fail the tests when they regress, and smokes the same idle cap on live Redis and Dragonfly.

Do we have a high-confidence way to reproduce? Yes: hold fake starts 16 overlapping Gets then asserts idle ≤ 8 and open sockets equal idle; in-flight Close then empty idle and no redial; Pester 16-way `/redis` and `/dragonfly` then bare CLIENT LIST remaining ≤ 8.

Is this the best way to solve the issue? Yes versus DestBranch: prove idle ≤ 8 after overlap and in-flight Close, and extend `/redis` `/dragonfly` overlap; do not take perf-01's total connection cap here.

### Evidence
What I checked:
- `go test -timeout 2m -count=1 ./...` passed
- `./Test-Integration.ps1` 10 passed / 0 failed (Redis and Dragonfly overlap Its green; reclaim green)
- `openspec validate test-idle-cap-and-release-after-close --strict` valid
- PR 14 OPEN, comments empty
- CI run 34651786131 Lint/Test/Integration Tests success

### Rank-up moves
None.
