Developer review: needs changes — 2026-09-13T16:03:02Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Explore mapped 18 dest-detectable slog events onto existing decision sites. No `Config.Logger` on dest yet. Two leftover-RESP events wait on PR #69.

**End users.** None.

## Motivation
SimpleRedis already classifies AUTH rejection, pool wait, truncated bulk, malformed RESP, handshake failure, NOSCRIPT reload, over-free, and a panic inside `do`. Dest returns those as errors (or lets the panic reach Traefik) and writes no slog line. `reclaim` already emits `reclaim_*` on a `*slog.Logger`. This package does not.

On dest, `New` copies host, password, pool, and retry knobs and freezes them. There is no logger field. `runOnConn` releases the socket on panic (PR #71) but does not log. `freeInUseTurn` increments `OverFrees` and continues. AUTH `WRONGPASS` becomes `redis:noauth` with no event. A waiter past `PoolTimeout` becomes `redis:unreachable` with no event. Under load, operators watching Traefik logs cannot tell a defended peer glitch from a broken in-use-turn invariant.

Until merge, production SimpleRedis misbehavior stays silent. A discard-handler default on every `Get` would also regress interpreted alloc (`interpretedcost_test.go`) even when nobody is listening.

```mermaid
sequenceDiagram
    participant App
    participant Client
    participant Peer
    App->>Client: Get
    Client->>Peer: AUTH then GET
    Peer-->>Client: WRONGPASS
    Client-->>App: redis:noauth
    Note over Client: dest writes no slog line
```

## Merge readiness
Explore recorded detection sites and assumed attribute strings. Product logging has not landed. Lint failed on the stub. 4 items remain.

Priority: P2 — operators cannot diagnose dest SimpleRedis failures that the library already classifies
Reviewed head: c5d5985
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 2/6 | Stub CI Lint failed; product logging not applied |
| CI proof | 2/6 | Lint failed, other jobs succeeded — [run 34767193478](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34767193478) |
| Local tests proof | N/A | localTests none, before implement |
| Review resolution | 6/6 | OPEN PR, no comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-structured-logging pushed | `git` origin (this card commit still to push) |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/74 | pr-host List |
| CI | build 34767193478 failed (Lint) https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34767193478 | GitHub check runs |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
- [ ] [Add leftover-RESP log events after PR #69](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-structured-logging/knowledge/debt/2026-09-13-simpleredis-leftover-resp-log-events.md) — `MsgSocketPoisoned` and `MsgAuthLeftover` have no dest detection site until PR #69 merges.

## How this fits together
Local spec is the dump. Branch `2026-09-13-simpleredis-structured-logging` from `origin/master` is PR 74. Explore wrote `devstate/explore.md`. Next is propose.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Exact MsgOpen attribute set (frozen knobs besides never Pass)? | additive asked | assumed — host, database, pool_size, max_idle_conns, pool_timeout, idle_timeout, dial_timeout, io_timeout, max_retries, min_retry_backoff, max_retry_backoff. Never Pass. | explore |
| Exact reason strings for MsgDial and MsgSocketClosed? | additive asked | assumed — idle_miss / stale; cancel / idle_cap | explore |
| Whether MsgNoAuth error is the Redis payload or redis:noauth? | additive asked | assumed — err.Error() after mapping (redis:noauth) | explore |
| Whether short-bulk read is bytes received before the short ReadFull? | additive asked | assumed — capture n from ReadFull; announced is RESP $ length; read is n | explore |
| Whether MsgCapability path is native/lua or the groupWritePath names? | additive asked | assumed — native or lua, logged when storeGroupWrite records the cache | explore |
| Whether non-per-command Debug (MsgOpen / MsgClose) still needs the Enabled guard? | additive asked | assumed — still wrap with Enabled; cost test targets per-command Debug sites | explore |
| MsgSocketClosed idle-cap is decided inside release; where does that event go? | additive asked | assumed — emit on the existing idle-cap branch inside release; do not change the signature | explore |
| Caller DeadlineExceeded — MsgTimeout or silence? | additive asked | assumed — MsgTimeout only for errTimeout; MsgCanceled only for Canceled; caller deadline stays silent | explore |

## Before merge
- [ ] [P2] Ship optional `Config.Logger` and the 18 dest-detectable events (not leftover-RESP)
- [ ] Tests: capturing handler, nil logger, secrets, Debug alloc=0, Yaegi, panic log then re-raise
- [ ] Merge last after OPEN #69, #72, and #73
- [ ] Stub Lint failure: https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34767193478/job/103750162411

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
| Reviewed head | c5d5985d04306ad2158ea083133eb62689c5c147 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest still has no slog in `simpleredis`. The agreed shape is reclaim's `Msg*` constants plus optional `Config.Logger`, with call-site nil/Enabled guards, not a discard handler.

Do we have a high-confidence way to reproduce? Yes — dest `simpleredis/` has zero `log/slog` imports; reclaim `table.go` already shows the convention; leftover-RESP sites are absent until PR #69.

Is this the best way to solve the issue? Yes — freeze `Logger` at `New` like other knobs, emit at the 18 dest-detectable decision sites, re-panic after `simpleredis_panic`, and defer the two leftover events.

### Evidence
What I checked:
- dest `Config` has no Logger (`simpleredis/config.go`, `origin/master` `c14cbec`)
- dest `runOnConn` defers release only (`simpleredis/commands_exec.go`); panic test recovers in the test (`panic_safety_test.go`)
- dest `replyError` maps WRONGPASS/NOAUTH/NOPERM/`ERR Client sent AUTH` (`simpleredis/resp.go`)
- dest `do` has no `Buffered()` check (`simpleredis/resp.go`); PR #69 still OPEN
- dest `release` idle-cap close is the only idle_cap detection site (`simpleredis/pool.go`)
- reclaim `Open` rejects nil logger; this ticket requires accepting nil
- PR 74 Lint failed, other checks succeeded (GitHub check runs, run 34767193478)

### Rank-up moves
None.
