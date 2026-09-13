Developer review: in progress — 2026-09-13T15:36:31Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `simpleredis-idle-mutex-defer` is apply-ready: `parkIdleConn` extracts the idle keep-or-close lock so `defer` owns the unlock, then `release` closes and returns the turn. Product apply is not on dest yet.

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
Propose artifacts are on the branch. Product apply is not. 5 items remain.

Priority: P1 — a recovered panic inside `release` deadlocks the idle mutex for the process lifetime
Reviewed head: 2b9519e
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Propose committed; product apply not started |
| CI proof | 1/6 | not seen on 2b9519e |
| Local tests proof | N/A | `localTests: none` (before implement) |
| Review resolution | 6/6 | OPEN PR #73, no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-idle-mutex-defer pushed | `git` HEAD 2b9519e |
| OpenSpec | simpleredis-idle-mutex-defer | `openspec/changes/simpleredis-idle-mutex-defer/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/73 | GitHub |
| CI | not seen | GitHub check runs after propose push |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | comments: none |

## Specs
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-idle-mutex-defer/openspec/changes/simpleredis-idle-mutex-defer/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local spec → branch `2026-09-13-simpleredis-idle-mutex-defer` from `origin/2026-09-13-simpleredis-panic-safe-release` → GitHub PR #73 (stacked on #71, must merge after it) → OpenSpec `simpleredis-idle-mutex-defer`.

**Stacked:** this PR is stacked on #71 and MUST merge after it. Base is `2026-09-13-simpleredis-panic-safe-release`, not master.

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
- [x] Explore: extract `parkIdleConn`; no panic-inside-lock test; fold tcp-session delta
- [x] Propose: OpenSpec `simpleredis-idle-mutex-defer` apply-ready

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
| Reviewed head | 2b9519e9db3433e1fd69777dd50ab97a0387b6b3 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: not applied versus dest yet. Propose keeps the extract: `parkIdleConn` owns the lock and defer; `release` closes and frees the turn after it returns.

Do we have a high-confidence way to reproduce? Not as a live panic (no seam without a production hook). Dest `release` has two manual unlocks after `idleConnsMu.Lock()`.

Is this the best way to solve the issue? Yes — extract so defer owns the unlock. A top-of-`release` `defer Unlock()` is a regression.

### Evidence
What I checked:
- `openspec validate simpleredis-idle-mutex-defer --strict` valid
- FindSpecHost fold `std_go_simpleredis_tcp-session` (high)
- `validate_artifact_names` OK

### Rank-up moves
None.
