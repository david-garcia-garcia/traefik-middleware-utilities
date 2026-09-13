Developer review: in progress — 2026-09-13T05:59:42Z

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
Stub PR is open. No product delta versus `origin/master` yet. Explore through implement remain.

Priority: P1 — Production is unsafe, or serving a wrong public contract today
Reviewed head: 9ec2fbf
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Pushed; CI not seen; prepare only |
| CI proof | 1/6 | Pushed and still not seen |
| Local tests proof | N/A | prHost github; CI covers it |
| Review resolution | 6/6 | OPEN PR; no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-bug-hook-panic-busy pushed | `git` / GitHub |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/49 | GitHub MCP |
| CI | not seen | pr-host CI |
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
None.

## Before merge
- [ ] [P1] Recover nil `create` and panics in create/Sleep/Wake/Close so a key is not left `slotBusy` and AfterFunc cannot crash the process
- [ ] Land product tests that fail on current master, then implement, then they pass
- [x] Stub review PR open

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
| Reviewed head | 9ec2fbfa02ebaee21739b812869c575a28dc7d18 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Not applied yet versus `origin/master`. Prepare recorded the recover protocol; implement has not landed it.

Do we have a high-confidence way to reproduce? Yes, the four entry points (create panic, nil create, Sleep panic, Wake panic) are named with expected hang / leftover / process-crash outcomes.

Is this the best way to solve the issue? Not applied yet. The agreed recover is at the table that owns `slotBusy`/`ready`, not a silent swallow inside `runSleep`/`runWake`/`runClose`.

### Evidence
What I checked:
- `reclaim/` exists on `origin/master` (`1aee4b8`)
- `put` / `drop` / `reclaimLocked` / `dispose` / `Reset` have no recover (`reclaim/table.go`, `1aee4b8`)
- One OPEN PR #49; comment inventory empty (GitHub MCP)

### Rank-up moves
None.
