Developer review: in progress — 2026-09-13T14:54:39Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

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
Explore reproduced the dest brick and kept the prototype design. Product fix is not on this branch yet. 4 items remain.

Priority: P1 — dest is permanently unreachable after PoolSize recovered panics
Reviewed head: 16b0c56
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Stub CI succeeded; product still to land |
| CI proof | 6/6 | succeeded, [build 34763834285](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34763834285) |
| Local tests proof | N/A | localTests none, before implement |
| Review resolution | 6/6 | OPEN PR, no comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-panic-safe-release pushed | `git` HEAD 16b0c56 |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/71 | GitHub |
| CI | build 34763834285 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34763834285 | Unit, Unit race, Lint, Go E2E Redis, Go E2E Dragonfly, Integration Tests, Integration Tests Redis, Integration Tests Dragonfly |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | comments: none |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-13-simpleredis-panic-safe-release` → PR 71 → CI 34763834285 green on prepare commits. Explore reproduced dest; apply not started.

## Explore Decisions
None.

## Before merge
- [ ] [P1] Land `runOnConn` deferred release from `origin/proto-defer-net` without redesign
- [ ] [P1] Land `borrow` `handedOff` and drop the explicit error-path `freeInUseTurn` calls
- [ ] Permanent panic-in-`do` test and Yaegi defer-on-panic probe (no `zz_` / proto / scratch names)
- [ ] Update `resp.go` `do` comment, tcp-session spec, and `std_go_simpleredis.md` gotcha

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
| Reviewed head | 16b0c568d4de219ce7798fe9452591bf2e99181c | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest still loses the turn and fd on a recovered panic. Copy `runOnConn` plus `borrow` `handedOff` from `origin/proto-defer-net` onto current dest; do not merge that branch (it predates #68 and would delete `BUGS.md`).

Do we have a high-confidence way to reproduce? Yes. Throwaway dest test (deleted after): PoolSize 2, two recovered panics inside `do` via panicking `bufio.Writer` → `turns=0/2 idle=0 OverFrees=0 accepts=2`, next Get `redis:unreachable`.

Is this the best way to solve the issue? Yes versus dest: prevent the leak with deferred release. Closed #66 / #70 refilled turns without closing the panicked fd; this ticket forbids that machinery.

### Evidence
What I checked:
- dest leak reproduced: `turns=0/2 idle=0 OverFrees=0 accepts=2`, Get `redis:unreachable` (`go test -v -count=1 -run TestExploreDestPanicInDoLosesTurn ./simpleredis/`)
- dest `exec` comments PR #29 and calls `release` only after `do` (`simpleredis/commands_exec.go`)
- dest `borrow` explicit `freeInUseTurn` on closed-idle and dial-error only (`simpleredis/pool.go`)
- dest `do` comment still says a panic loses the turn (`simpleredis/resp.go`)
- dest pins `github.com/traefik/yaegi v0.16.1` (`go.mod`); compose `traefik:v3.7.11`; Traefik v3.7.11 `go.mod` requires the same (`knowledge/research/ext_traefik_plugins_yaegi-generics/.sources/traefik-v3.7.11-go.mod.md`)
- prototype `runOnConn` / `handedOff` on `origin/proto-defer-net`; do not merge that branch onto dest
- Yaegi pin resolved, not assumed. Explore Decisions empty.

### Rank-up moves
None.
