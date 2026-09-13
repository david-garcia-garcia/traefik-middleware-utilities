Developer review: in progress — 2026-09-13T15:33:18Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Motivation
On dest, `release` still unlocks `idleConnsMu` by hand on two paths after the keep-or-close decision. A panic in that section leaves the mutex locked for the process lifetime. Traefik recovers the panicking plugin request, so the process stays up. Every later `borrow` waits in `takeIdleConn`, every later `release` waits, and `Close` waits. Stuck goroutines accumulate. That is worse than the in-use-turn leak #71 already defers around.

#71 already defers `release` in `runOnConn` and `freeInUseTurn` in `borrow`. Dest still hand-unwinds this one mutex. A naive `defer Unlock()` at the top of `release` would hold the lock across `conn.close()` and the `freeInUseTurn` channel send — a different production stall. The extract-and-defer shape (`parkIdleConn`) is the remaining apply.

If this PR does not land, dest keeps a permanent deadlock on a recovered panic inside `release`, and `BUGS.md` section 2 still tells the next reader that `exec` releases without `defer`.

```mermaid
sequenceDiagram
  participant Panic as panic inside release
  participant Mutex as idleConnsMu
  participant Borrow as later borrow
  participant Close as Close
  Panic->>Mutex: Lock
  Panic-->>Mutex: never Unlock
  Borrow->>Mutex: Lock waits forever
  Close->>Mutex: Lock waits forever
```

## Merge readiness
Explore recorded the extract shape and the four frozen semantics. Product apply is not on this branch yet. 5 items remain.

Priority: P1 — a recovered panic inside `release` deadlocks the idle mutex for the process lifetime
Reviewed head: e8c39e5
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Explore committed; CI on that commit not seen yet |
| CI proof | 1/6 | not seen on e8c39e5; prior stub run 34765760774 succeeded |
| Local tests proof | N/A | `localTests: none` (before implement) |
| Review resolution | 6/6 | OPEN PR #73, no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-idle-mutex-defer pushed | `git` HEAD e8c39e5 |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/73 | GitHub |
| CI | not seen | GitHub check runs after explore push |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | comments: none |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-13-simpleredis-idle-mutex-defer` from `origin/2026-09-13-simpleredis-panic-safe-release` → GitHub PR #73 (base that dest, stacked on #71, must merge after it) → explore recorded.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Can a panic-inside-`parkIdleConn` test be written with no production API change? | additive asked | assumed — no. Existing pool reuse, live-under-cap, OverFrees, cancel-close, and panic-in-`do` tests cover the four semantics. A panic inside the locked section would need a production hook, which the requirement forbids. | explore |

## Before merge
- [ ] [P1] Extract `parkIdleConn` so `release` unlocks `idleConnsMu` via defer without holding the lock across close or `freeInUseTurn`
- [ ] Defer the two `groupWriteMu` unlocks in `cachedGroupWrite` / `storeGroupWrite`
- [ ] Replace `simpleredis/BUGS.md` section 2 with the Yaegi-defer finding and point at `simpleredis/yaegi_defer_test.go`
- [ ] Full local suite including Yaegi tests, `go vet`, pool/release tests `-count=5`, measured CI green
- [ ] Merge after #71 (`2026-09-13-simpleredis-panic-safe-release`)
- [x] Stub PR #73 opened with base `2026-09-13-simpleredis-panic-safe-release`
- [x] Explore: extract `parkIdleConn`; no panic-inside-lock test; fold tcp-session delta; leave Close / takeIdleConn / AfterFunc / OverFrees

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
| Reviewed head | e8c39e58d0b3be4944c10e1be3f7cc02266545dd | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: not applied versus dest yet; dest still hand-unlocks `idleConnsMu` in `release`. Explore keeps the extract: `parkIdleConn` owns the lock and defer, `release` closes and frees the turn after it returns.

Do we have a high-confidence way to reproduce? Not as a live panic (no seam without a production hook). Dest `release` has two manual unlocks after `idleConnsMu.Lock()` with close and `freeInUseTurn` after unlock (`simpleredis/pool.go`). That is Go mutex semantics.

Is this the best way to solve the issue? Yes — extract the locked decision so defer owns the unlock, then close and free the turn after return. A top-of-`release` `defer Unlock()` is a regression.

### Evidence
What I checked:
- Dest `simpleredis/pool.go` `release` two manual unlocks; `takeIdleConn` already defers unlock; `Close` already closes sockets outside the lock
- One `release` call site: `commands_exec.go` `runOnConn`
- `simpleredis/commands_msetex.go` two manual `groupWriteMu` unlocks
- `simpleredis/BUGS.md` section 2 still says exec releases without defer; ticket quote not found
- `knowledge/devdocs/std_go_simpleredis.md` and `openspec/specs/std_go_simpleredis_tcp-session/spec.md` already have the Yaegi-defer finding; no stale claim
- `simpleredis/resp.go` / `commands_exec.go` stale #29 comments gone
- In-flight `2026-09-13-simpleredis-close-abandoned-socket` `release` adds `deregisterCheckout` then dest's body
- Prior stub CI run 34765760774 all 8 checks success (before this explore commit)

### Rank-up moves
None.
