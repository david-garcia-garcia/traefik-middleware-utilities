## Motivation
When a key trips OPEN, the gate waits out the cooldown and then admits one HALF-OPEN probe. That probe's Report is the only outcome that may close the key, reopen it, or change n. Cooldown math is already specified to use only that probe result.

Allow records only a boolean probeOutstanding. Report takes the key and a success flag, with no identity of the attempt. In HALF-OPEN the first Report to arrive is treated as the probe, even when it belongs to a request admitted before the trip.

With TripFailures 5, Jitter 0, and BaseCooldown 1s: six requests are admitted while CLOSED; five fail fast and trip OPEN while the sixth is still in flight; the cooldown elapses and probe P is admitted; the sixth request then Reports success and the gate closes with full credit; the real probe's failure only subtracts one credit. The key is CLOSED, credit 4, n 0, although the probe failed. The same steal happens when a probe lease expires: a replacement probe is admitted and the late Report of the first one takes its slot.

Left alone, concurrency reopens an unhealthy backend (flapping onto it) or keeps a healthy one OPEN with n inflated toward MaxCooldown.

Priority: P2 — real operator and end-user pain from a stale report deciding the probe, with limited blast radius

## Implementation
Allow returns a Permit that carries the opaque key and that entry's generation; a deny is the zero Permit, whose Allowed() is false. Report takes that Permit instead of a key. Each entry holds a generation from a gate-wide counter, assigned when the slot is created and bumped on every state transition (trip, OPEN to HALF-OPEN, a replacement probe after an expired lease, HALF-OPEN close or reopen). CLOSED admits do not bump, so every in-flight attempt of the current CLOSED era still reports. Report applies only when the permit's generation matches the entry; a mismatch, a zero permit, or a permit from an expired then recreated key is a no-op (no credit, state, n, or TTL change). There are no legacy Allow/Report signatures.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** `Allow` returns a `Permit`; `Report` takes that permit instead of the key; `Allowed()` replaces the bool; a permit from another generation is ignored.
**End users.** A HALF-OPEN outcome is no longer stolen by an in-flight request from before the probe, so an unhealthy backend is not reopened by a stale success and a healthy one is not held OPEN by a stale failure.

## Merge readiness
In progress. 0 items remain.

Priority: P2 — real operator and end-user pain from a stale report deciding the probe, with limited blast radius
Reviewed head: c8e64853
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Not ready |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-backendbackoff-stale-probe-report pushed | `git` |
| OpenSpec | backendbackoff-probe-permit | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/121 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_backendbackoff_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/openspec/changes/archive/2026-11-22-backendbackoff-probe-permit/proposal.md) — modified
- [std_go_backendbackoff_cooldown](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/openspec/changes/archive/2026-11-22-backendbackoff-probe-permit/proposal.md) — modified

Completed:
- [std_go_backendbackoff_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/openspec/specs/std_go_backendbackoff_allow/spec.md) — modified
- [std_go_backendbackoff_cooldown](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/openspec/specs/std_go_backendbackoff_cooldown/spec.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-backendbackoff-stale-probe-report on branch 2026-11-22-backendbackoff-stale-probe-report targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/121; CI not seen.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/devstate/2026/11/2026-11-22-backendbackoff-stale-probe-report/codereview_standards.md) — 3 total, 0 pending, 3 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/devstate/2026/11/2026-11-22-backendbackoff-stale-probe-report/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/devstate/2026/11/2026-11-22-backendbackoff-stale-probe-report/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/devstate/2026/11/2026-11-22-backendbackoff-stale-probe-report/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/devstate/2026/11/2026-11-22-backendbackoff-stale-probe-report/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/devstate/2026/11/2026-11-22-backendbackoff-stale-probe-report/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/devstate/2026/11/2026-11-22-backendbackoff-stale-probe-report/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-backendbackoff-stale-probe-report/devstate/2026/11/2026-11-22-backendbackoff-stale-probe-report/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 4 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | c8e64853052aa0bdd3e718d0af92bb1a008f672c | Card must match the branch you measured |

### Stored data model
None.
