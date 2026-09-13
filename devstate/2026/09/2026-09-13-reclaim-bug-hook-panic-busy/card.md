Developer review: in progress — 2026-09-13T06:08:53Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `reclaim-hook-panic-recovery` folds nil-create/create-panic into `std_go_reclaim_context-lease` and panicking Sleep/Wake/Close into `std_go_reclaim_value-lifecycle`. Table recover is not applied yet.

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
Proposal is on the branch. Implement remains (tests that fail on master, then recover).

Priority: P1 — Production is unsafe, or serving a wrong public contract today
Reviewed head: 1bf59d3
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Pushed; CI on this head not seen; propose only |
| CI proof | 1/6 | Pushed and still not seen on 1bf59d3 |
| Local tests proof | N/A | prHost github; CI covers it |
| Review resolution | 6/6 | OPEN PR; no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-bug-hook-panic-busy pushed | `git` / GitHub |
| OpenSpec | reclaim-hook-panic-recovery | `openspec/changes/reclaim-hook-panic-recovery/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/49 | GitHub MCP |
| CI | not seen | head 1bf59d3 just pushed |
| Local tests | none | handoff.yaml |
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
- [ ] [P1] Recover nil `create` and panics in create/Sleep/Wake/Close so a key is not left `slotBusy` and AfterFunc cannot crash the process
- [ ] Land product tests that fail on current master, then implement, then they pass
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
| Reviewed head | 1bf59d34ed8095c336738a199e76074d53f1aba4 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Not applied yet versus `origin/master`. Recover at `put` / `drop` / `reclaimLocked` / `dispose` / `Reset`.

Do we have a high-confidence way to reproduce? Yes. Explore throwaway tests: 2a–2d hang / leftover busy / AfterFunc child `exit status 2`.

Is this the best way to solve the issue? Not applied yet. The agreed recover is at the table that owns `slotBusy`/`ready`.

### Evidence
What I checked:
- Change `reclaim-hook-panic-recovery` validates (`openspec validate --strict`)
- FindSpecHost fold into `std_go_reclaim_context-lease` and `std_go_reclaim_value-lifecycle`
- One OPEN PR #49

### Rank-up moves
None.
