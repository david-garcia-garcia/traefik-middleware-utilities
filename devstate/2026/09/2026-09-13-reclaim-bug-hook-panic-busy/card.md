Developer review: in progress — 2026-09-13T06:16:06Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `reclaim.Table` recovers a nil `create` and panics in create, Sleep, Wake, and Close so a key is not left `slotBusy` and AfterFunc cannot crash the process. `Open` can return a wrapped Wake panic error. Tests 2a–2d.

**End users.** None.

## Motivation
The reclaim table keeps one value per key across Traefik holder contexts, with optional Sleep, Wake, and Close hooks. Those hooks and `create` run while the key is parked `slotBusy` and waiters sit on `ready`.

On `origin/master`, a panic in `create`, a nil `create`, a panic in Sleep, or a panic in Wake never closes `ready`, never unmaps the key, and never runs Close. The next `Open` for that key hangs. A Sleep panic on the production `AfterFunc` path is an unrecovered goroutine panic and kills the process.

If we do not merge the recover protocol, one bad callback bricks the key for every later caller and a Sleep panic can take down the Traefik process.

```mermaid
sequenceDiagram
  participant O as Open
  participant T as Table
  participant H as create or hook
  O->>T: map key slotBusy, ready open
  T->>H: create / Sleep / Wake
  H--xT: panic or nil create
  Note over T: ready never closed, key still mapped
  O->>T: later Open waits on ready
  Note over O: hang; AfterFunc Sleep panic crashes process
```

## Merge readiness
Recover is on the branch. Local unit tests passed. CI on this head is still queued.

Priority: P1 — Production is unsafe, or serving a wrong public contract today
Reviewed head: 2df37dc
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI in progress on the recover head |
| CI proof | 3/6 | queued — https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742235082 |
| Local tests proof | N/A | prHost github; CI covers it |
| Review resolution | 6/6 | OPEN PR; no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-bug-hook-panic-busy pushed | `git` / GitHub |
| OpenSpec | reclaim-hook-panic-recovery | `openspec/changes/reclaim-hook-panic-recovery/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/49 | GitHub MCP |
| CI | build 34742235082 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742235082 | GitHub MCP check runs queued |
| Local tests | passed | `go test -short -timeout 2m -count=1 ./...` |
| PR comments | no comments | comments none |

## Specs
- [std_go_reclaim_context-lease](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-bug-hook-panic-busy/openspec/changes/reclaim-hook-panic-recovery/proposal.md) — modified
- [std_go_reclaim_value-lifecycle](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-reclaim-bug-hook-panic-busy/openspec/changes/reclaim-hook-panic-recovery/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket `devstate/2026/09/2026-09-13-reclaim-bug-hook-panic-busy/` on branch `2026-09-13-reclaim-bug-hook-panic-busy`; durable card is PR #49 summary.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| What wrap string for Sleep / Wake / Close panics? | additive asked | assumed — same shape as create (`reclaim: sleep/wake %q: panic: %v`); Close recover only | explore |
| What error for nil `create`? | additive asked | assumed — `fmt.Errorf("reclaim: create %q: nil create", key)` | explore |
| Do concurrent waiters on a panicking Wake receive `createErr`? | additive asked | assumed — set `createErr` like create failure; later Open creates | explore |
| After a Sleep panic, emit `reclaim_orphan` / `reclaim_dispose`? | additive incidental | assumed — skip orphan (Sleep did not return); still Close then dispose | explore |

## Before merge
- [x] [P1] Recover nil `create` and panics in create/Sleep/Wake/Close so a key is not left `slotBusy` and AfterFunc cannot crash the process
- [x] Land product tests that fail on current master, then implement, then they pass
- [ ] CI green on 2df37dc
- [x] Stub review PR open
- [x] Explore: 2a–2d reproduced on master
- [x] Propose `reclaim-hook-panic-recovery`

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
| Reviewed head | 2df37dc32b11cdaa347bb6ae1a097a947ffcb817 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Recover at `put` / `drop` / `reclaimLocked` / `dispose` / `Reset`, sharing `failBusySlot` with the existing create-error path. `runSleep` / `runWake` / `runClose` stay thin.

Do we have a high-confidence way to reproduce? Yes. Fail-then-pass: 2a/2b hung 10s (`Open did not return; key left busy`); 2c child `exit status 2` from AfterFunc; 2d Close never ran. After `2e24a03` those four tests pass in 0.19s; `go test -short ./...` passed.

Is this the best way to solve the issue? Yes versus `origin/master`. The table owns `slotBusy`/`ready`; a silent swallow in the runners would look like success.

### Evidence
What I checked:
- Fail: `go test -run TestTable_CreatePanicUnsticksKey|...` on `1baff53` (tests only) — 4 FAIL
- Pass: same four tests after `2e24a03`; `go test -short -timeout 2m -count=1 ./...` passed
- CI run 34742235082 queued (GitHub MCP)

### Rank-up moves
None.
