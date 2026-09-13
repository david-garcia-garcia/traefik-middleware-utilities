Developer review: in progress — 2026-09-13T06:01:02Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Motivation
On dest, `Table.Open` is the call that both creates (or rebinds) a keyed value and attaches the caller’s holder context. `(value, nil)` is how callers know they own a live pointer.

At zero grace, canceling that holder context before `Open` returns can run Close on the same stack via `AfterFunc` while `Open` still returns the pointer. The reproduced path is cancel during a blocking `create`; the same bind-then-drop race exists on an already-awake bind and on reclaim. A caller that type-asserts and uses that pointer is talking to an incarnation that already ended.

If this stays unmerged, any host that cancels during a slow create (or opens with an already-done context) can observe a closed value with no error. The agreed contract is `(value, nil)` only when the holder was still live at return; otherwise `(nil, ctx.Err())` and `drop` on this stack.

```mermaid
sequenceDiagram
  participant Caller
  participant Open
  participant create
  participant drop
  Caller->>Open: Open(canceled-during-create)
  Open->>create: blocking
  Note over create: holder ctx canceled
  create-->>Open: value, nil error
  Open->>drop: AfterFunc fires now
  drop-->>drop: zero grace Sleep then Close
  Open-->>Caller: pointer, nil
```

## Merge readiness
Prepare is grounded; product apply has not started. Explore is next.

Priority: P1 — serving a wrong public contract today
Reviewed head: 02c24e1
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI still running; no product apply yet |
| CI proof | 3/6 | Checks in progress on the stub PR |
| Local tests proof | N/A | Before implement (`localTests: none`) |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-reclaim-bug-canceled-open-close pushed | `git push` `02c24e1` |
| OpenSpec | none | no change folder |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/51 | pr-host Create |
| CI | build 34741603037 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34741603037 | pr-host CI (head at check was `82bfbb2`; bus commit `02c24e1` pushed after) |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-reclaim-bug-canceled-open-close` from `origin/master` (reclaim present; `origin/HEAD` is stale `initial`). Stub PR #51 is the durable card. Next is explore.

## Explore Decisions
None.

## Before merge
- [ ] Land reclaim tests that fail on dest for cancel-during-create, already-done awake bind, and already-done reclaim, then apply bind-time `ctx.Err()` drop at those three sites
- [ ] Fold the canceled-bind contract into the reclaim spec and usage packet

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
| Reviewed head | 02c24e1702835688def220616b76ba77d5191f59 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Dest still returns `(value, nil)` after Close can already have run; the ticket’s bind-time `ctx.Err()` drop at put, awake bind, and reclaimLocked is the how.

Do we have a high-confidence way to reproduce? Yes, cancel during blocking create at zero grace, as in the ticket example.

Is this the best way to solve the issue? Yes — delaying AfterFunc until after return still hands the caller a pointer whose Close may already have run; a pre-create check misses cancel during create.

### Evidence
What I checked:
- `reclaim/` exists on `origin/master` at `1aee4b8` (`git ls-tree`)
- `put` / `Open` awake / `reclaimLocked` / `dropWhenDone` (`reclaim/table.go`)
- Dest tests cancel after return or race a second live Open (`reclaim/table_test.go`)
- Stub PR #51, comment inventory empty, CI run 34741603037 in progress

### Rank-up moves
None.
