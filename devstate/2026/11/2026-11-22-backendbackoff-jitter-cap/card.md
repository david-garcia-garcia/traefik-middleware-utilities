## Motivation
After a trip, the backendbackoff gate waits `BaseCooldown * 2^n` (already capped at `MaxCooldown`) and then jitters that wait so replica probes do not land together. `Jitter` 0 freezes the wait; a positive `Jitter` is the spread.

The spread was `wait * (1 + J * (2u - 1))` with `u` in `[0, 1)`, then a clamp onto `MaxCooldown`. Once `wait` is already the cap, every draw with `u > 0.5` overshoots and returns exactly `MaxCooldown`: 50.2% of 100,000 at-cap samples piled on that one Duration. The same clamp fires when `wait` is still below the cap but `wait * (1 + J)` is not: with `BaseCooldown` 9.5s, `MaxCooldown` 10s, `Jitter` 0.1, 23.9% of first-trip waits landed on the max.

Left alone, a long shared outage drives every replica to the cap. R replicas and K keys then send about `R*K/2` probes on the same instant, each cycle, at the backend that is trying to recover. That is the opposite of what `Jitter` is for.

Priority: P1 — production replica probes lockstep at MaxCooldown today, a wrong jitter contract

## Implementation
`cooldownDuration` still doubles `BaseCooldown` up to `MaxCooldown`. When `Jitter` is greater than zero it now builds the interval first — `lower = wait * (1 - J)`, `upper = min(wait * (1 + J), MaxCooldown)` — and draws uniformly inside it (`lower + u * (upper - lower)`, `u` in `[0, 1)`). There is no clamp after the sample. When the +J side already fits under the cap, the interval is the old ±J band. When `upper` is the cap, the half-open draw stays below `MaxCooldown`, so the cap is an endpoint of the range, not a pile-up. `Jitter` 0 still returns `wait` exactly. The cooldown spec states that interval; proofs cover at-cap, near-cap, below-cap, and the zero-jitter control.

## What this changes
**Operators.** Replica probes at a capped cooldown spread instead of landing together on MaxCooldown; existing Jitter and MaxCooldown knobs, no new deploy key.
**Admin users.** None.
**Developers.** Jittered cooldown is uniform on `[wait*(1-J), min(wait*(1+J), MaxCooldown))`; Jitter 0 is still exact wait; Allow/Report call shape is unchanged.
**End users.** None.

## Merge readiness
Ready for review. 0 items remain.

Priority: P1 — production replica probes lockstep at MaxCooldown today, a wrong jitter contract
Reviewed head: 5f3d2b5e
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/45014123617 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-backendbackoff-jitter-cap pushed | `git` |
| OpenSpec | backendbackoff-jitter-below-cap | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/124 | pr-host |
| CI | build 45014123617 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/45014123617 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/45014123617 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_backendbackoff_cooldown](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/openspec/changes/archive/2026-11-22-backendbackoff-jitter-below-cap/proposal.md) — modified

Completed:
- [std_go_backendbackoff_cooldown](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/openspec/specs/std_go_backendbackoff_cooldown/spec.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-backendbackoff-jitter-cap on branch 2026-11-22-backendbackoff-jitter-cap targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/124; CI build 45014123617 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/45014123617.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/devstate/2026/11/2026-11-22-backendbackoff-jitter-cap/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/devstate/2026/11/2026-11-22-backendbackoff-jitter-cap/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/devstate/2026/11/2026-11-22-backendbackoff-jitter-cap/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/devstate/2026/11/2026-11-22-backendbackoff-jitter-cap/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/devstate/2026/11/2026-11-22-backendbackoff-jitter-cap/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/devstate/2026/11/2026-11-22-backendbackoff-jitter-cap/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/devstate/2026/11/2026-11-22-backendbackoff-jitter-cap/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-jitter-cap/devstate/2026/11/2026-11-22-backendbackoff-jitter-cap/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 5f3d2b5e7563bf8fdabe4529992aa7204df4692d | Card must match the branch you measured |

### Stored data model
None.
