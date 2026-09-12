Developer review: in progress — 2026-09-12T12:49:33.514Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `windowcounter-buffered-flush-error` folds buffered outage error into `std_go_windowcounter_sliding-take` and `std_go_windowcounter_sync-flush`. Limiter code is still DestBranch until implement.

**End users.** None.

## Motivation
Buffered `Take` is supposed to share one Redis integer across instances, flushing local hits on `sync_rate`. On `master` that flush error is thrown away. While a delta is still local, the next `Take` never talks to Redis, so a dead server looks like a healthy admit.

Kill Redis after one buffered hit and the following Takes return a nil error. Each process still stops at its own `limit`; N processes therefore admit about `limit × N`. Exact mode (`sync_rate` 0) already returns `redis:unreachable`. The performance knob is also the failure-contract knob, and neither `New` nor the README says so.

Leaving this unmerged keeps a rate limiter that looks up from the middleware while the global cap is gone — worst on the hottest key, which almost always has a pending delta.

```mermaid
flowchart TD
  take[Buffered Take after Redis dies]
  delta{"localDelta == 0?"}
  getGet[GET window key]
  honest[Error returned — fail closed]
  silent[No Redis call — nil error, admit locally]
  take --> delta
  delta -->|yes, just flushed| getGet --> honest
  delta -->|no, between ticks| silent
```

## Merge readiness
Propose recorded the OpenSpec change; product limiter has not landed. 1 item remains.

Priority: P1 — production is serving a wrong public contract today: buffered Take admits without an error while Redis is down, so the shared limit does not hold.

Reviewed head: 7b40da2
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI queued on the propose commit; limiter still DestBranch |
| CI proof | 3/6 | run 34694733597 in progress |
| Local tests proof | N/A | before implement |
| Review resolution | 6/6 | OPEN PR, no comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-12-simpleredis-bug-05-windowcounter-hides-outage pushed | `git` / origin |
| OpenSpec | windowcounter-buffered-flush-error | `openspec/changes/windowcounter-buffered-flush-error/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/30 | pr-host List/Create |
| CI | build 34694733597 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34694733597 | Lint queued, Test queued, Go E2E queued, Integration Tests queued |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | none |

## Specs
- [std_go_windowcounter_sliding-take](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-12-simpleredis-bug-05-windowcounter-hides-outage/openspec/changes/windowcounter-buffered-flush-error/proposal.md) — modified
- [std_go_windowcounter_sync-flush](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-12-simpleredis-bug-05-windowcounter-hides-outage/openspec/changes/windowcounter-buffered-flush-error/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Branch `2026-09-12-simpleredis-bug-05-windowcounter-hides-outage`, stub PR #30, OpenSpec change `windowcounter-buffered-flush-error`, CI run 34694733597 queued.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Which of the three error surfaces does this run ship (Take error, LastFlushError/Stale poll, or staleness deadline)? | bounded asked | assumed — staleness k=1 plus return lastFlushErr from Take and Peek on the existing (bool, float64, error) slot; store lastFlushErr / flushFailedAt / lastRedisOK; probe with flushPending when stale; do not add LastFlushError or a fourth return; do not GET every buffered Take. | explore |
| What is the staleness multiplier k? | additive asked | assumed — k = 1 (one missed sync_rate interval). now.Sub(lastRedisOK) >= syncRate triggers one flushPending probe. | explore |
| Must buffered Peek with a pending delta surface the same error as Take? | bounded asked | assumed — Peek returns the same lastFlushErr / staleness error as Take. Do not leave Peek as a silent sibling. | explore |
| Is a third Take return value acceptable under Yaegi and existing callers? | additive asked | assumed — keep (bool, float64, error). Put the flush/staleness error in the existing error slot. Do not add a fourth return. Yaegi callers already unpack three values. | explore |
| What does Kong Advanced / OSS do when a buffered flush fails? | additive incidental | assumed — do not clone Kong for flush-fail; follow std_go_windowcounter_sliding-take Redis-errors-propagate. Kong sync_rate remains the accuracy knob only. | explore |

## Before merge
- [ ] [P1] Land staleness k=1 so buffered Take/Peek return lastFlushErr on Redis outage
- [x] OpenSpec change `windowcounter-buffered-flush-error` proposed
- [x] Explore recorded the surface (staleness k=1, existing error slot, Peek same as Take)
- [x] Stub PR #30 opened
- [x] Requirement written and qualified-with-gaps

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 7b40da2340a8a59a9fbeefb220b47125b66ae7a7 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: versus `master`, return the retained flush error from Take and Peek after one missed sync_rate, instead of discarding it and admitting locally.

Do we have a high-confidence way to reproduce? Yes, the ticket measured it: after one buffered hit with Redis killed, 11/11 Takes returned a nil error.

Is this the best way to solve the issue? Yes versus `master`: the OpenSpec change folds that contract into the two existing windowcounter leaves.

### Evidence
What I checked:
- FindSpecHost fold into `std_go_windowcounter_sliding-take` and `std_go_windowcounter_sync-flush` (high, existing leaves)
- `openspec validate windowcounter-buffered-flush-error --type change --strict` valid
- PR #30 OPEN; CI run 34694733597 Lint, Test, Go E2E, Integration Tests queued

### Rank-up moves
None.
