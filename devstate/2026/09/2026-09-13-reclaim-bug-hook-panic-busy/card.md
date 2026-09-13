Developer review: in progress — 2026-09-13T06:05:53Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

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
Explore recorded. No product delta versus `origin/master` yet. Propose through implement remain.

Priority: P1 — Production is unsafe, or serving a wrong public contract today
Reviewed head: 4a9cda6
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Pushed; CI on this head not seen; explore only |
| CI proof | 1/6 | Pushed and still not seen on 4a9cda6 |
| Local tests proof | N/A | prHost github; CI covers it |
| Review resolution | 6/6 | OPEN PR; no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-bug-hook-panic-busy pushed | `git` / GitHub |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/49 | GitHub MCP |
| CI | not seen | GitHub MCP check runs on previous SHA 8751428; head 4a9cda6 just pushed |
| Local tests | none | handoff.yaml |
| PR comments | no comments | comments none |

## Specs
None.

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
- [x] Explore: 2a–2d reproduced on master (hang / leftover busy / AfterFunc child `exit status 2`)

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 4a9cda6c09eb89bf11e25cec713d0ad1cd6a7784 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Not applied yet versus `origin/master`. Recover at `put` / `drop` / `reclaimLocked` / `dispose` / `Reset`, not a silent swallow inside `runSleep` / `runWake` / `runClose`.

Do we have a high-confidence way to reproduce? Yes. Throwaway tests on HEAD matching master: 2a create panic leftover `slotBusy` then second Open hung 200ms; 2b nil create same hang; 2c AfterFunc Sleep panic child `exit status 2`; 2d Wake panic then third Open hung and Close never ran.

Is this the best way to solve the issue? Not applied yet. The agreed recover is at the table that owns `slotBusy`/`ready`.

### Evidence
What I checked:
- `reclaim/` exists on `origin/master` (`1aee4b8`)
- Throwaway `TestThrowaway_*` then deleted (`go test -run ^TestThrowaway_`, 4a9cda6 tree)
- Previous CI on 8751428: Go E2E Redis failed; other checks success (`https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34741722649`)
- One OPEN PR #49; comment inventory empty (GitHub MCP)

### Rank-up moves
None.
