Developer review: in progress — 2026-09-13T15:03:37Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `exec` defers `release` via `runOnConn`. `borrow` defers `freeInUseTurn` unless the socket is handed off. A recovered panic returns the in-use turn and closes the socket. Permanent proofs: `simpleredis/panic_safety_test.go`, `simpleredis/yaegi_defer_test.go`.

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
Product is on the branch. CI on the product commit is not seen yet. 1 item remains.

Priority: P1 — dest is permanently unreachable after PoolSize recovered panics
Reviewed head: 86dda81
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Product pushed, CI not seen |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | remote PR; localTests passed |
| Review resolution | 6/6 | OPEN PR, no comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-panic-safe-release pushed | `git` HEAD 86dda81 |
| OpenSpec | simpleredis-panic-safe-release | `openspec/changes/simpleredis-panic-safe-release/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/71 | GitHub |
| CI | not seen | GitHub check runs empty immediately after push |
| Local tests | passed | `go vet ./simpleredis/` clean; `go test -count=1 -timeout 300s ./simpleredis/` 14.950s; `-count=5 -run "Pool|Panic|Release|Borrow|Turn"` 7.922s; no pre-existing test files modified |
| PR comments | no comments | comments: none |

## Specs
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-panic-safe-release/openspec/changes/simpleredis-panic-safe-release/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-13-simpleredis-panic-safe-release` → PR 71 → OpenSpec `simpleredis-panic-safe-release` → product HEAD 86dda81.

## Explore Decisions
None.

## Before merge
- [ ] CI on product commit 86dda81
- [x] [P1] Land `runOnConn` deferred release
- [x] [P1] Land `borrow` `handedOff` and drop the explicit error-path `freeInUseTurn` calls
- [x] Permanent panic-in-`do` test and Yaegi defer-on-panic probe
- [x] Update `resp.go` `do` comment and `std_go_simpleredis.md` gotcha

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
| Reviewed head | 86dda819410ba44cbf96f1738405545fddcc8d14 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest now defers `release` in `runOnConn` and `freeInUseTurn` in `borrow` unless handed off. Same design as `origin/proto-defer-net`, applied onto current dest (no merge of that branch).

Do we have a high-confidence way to reproduce? Yes. Dest throwaway: `turns=0/2` then `redis:unreachable`. After apply: `TestPanicInDoReturnsTurnAndClosesSocket` `turns=2/2 idle=0 accepts=3 OverFrees=0`.

Is this the best way to solve the issue? Yes versus dest: prevent the leak. Closed #66 / #70 refilled turns without closing the panicked fd.

### Evidence
What I checked:
- `go vet ./simpleredis/` clean
- `go test -count=1 -timeout 300s ./simpleredis/` passed in 14.950s
- `go test -count=5 -timeout 600s -run "Pool|Panic|Release|Borrow|Turn" ./simpleredis/` passed in 7.922s
- `git diff origin/master -- simpleredis/*_test.go` empty (no pre-existing test edited)
- `TestYaegi_DeferRunsOnPanic`: Eval errors `boom`, `errors: *target must be interface or implement error`, `assignment to entry in nil map`; turns=0 each
- Yaegi pin resolved: `go.mod` and Traefik v3.7.11 both require v0.16.1

### Rank-up moves
None.
