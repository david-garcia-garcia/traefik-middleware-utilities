Developer review: in progress — 2026-09-11T08:12:01Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Research notes for crowdsec SimpleRedis (`knowledge/research/ext_crowdsec_simpleredis/`) are on the branch; DestBranch still has no `redis/` library, no Yaegi Redis tests, and no Redis host plugin.

**End users.** None.

## Motivation
DestBranch lists Redis connection as Planned and has no `redis/` tree. Other Traefik plugins cannot import a shared client from `github.com/david-garcia-garcia/traefik-middleware-utilities`; they keep a private copy (crowdsec-bouncer `pkg/simpleredis` is the named source).

Without this change the Planned library never lands, Yaegi coverage never exists for that client, and each middleware keeps its own RESP dialer. This PR so far only grounds the ticket and records those source facts. Explore still has to pick `redis/` vs `simpleredis/` and how Pester talks to Redis.

## Merge readiness
Prepare is qualified-with-gaps. Explore has not started. 2 items remain.

Priority: P3 — missing planned library; no production harm today
Reviewed head: aeaf6bd
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI still in progress; no library code yet |
| CI proof | 3/6 | Lint, Test, and Integration Tests in progress — https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34577992980 |
| Local tests proof | N/A | `localTests: none`; remote CI is the proof axis |
| Review resolution | 6/6 | OPEN PR #3, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-simpleredis pushed | `git` / origin/2026-09-11-simpleredis |
| OpenSpec | none | `openspec/changes/` has no live change |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/3 | GitHub MCP Create |
| CI | build 34577992980 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34577992980 | GitHub check runs |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | empty Comment-List |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket 2026-09-11-simpleredis is the branch and the stub PR into master. Prepare researched the named SimpleRedis package and wrote the requirement; later phases still have to copy the library and prove it under Yaegi.

## Explore Decisions
None.

## Before merge
- [ ] Land the copied SimpleRedis library, Yaegi tests, Redis host plugin, and Pester e2e [P3]
- [ ] Decide `redis/` vs `simpleredis/` and how e2e Redis is provided [P3]
- [x] Stub PR #3 opened into master
- [x] Crowdsec SimpleRedis researched @ `6548da47`

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
| Reviewed head | aeaf6bd8990e824f87062ffff0356d4c8d4e5a0e | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Not applicable yet — DestBranch still has no Redis library; this PR only records source facts and the requirement.

Do we have a high-confidence way to reproduce? Yes, dest tree has no `redis/`; source package is pinned in research notes.

Is this the best way to solve the issue? Not decided — explore still owns folder name and e2e Redis shape.

### Evidence
What I checked:
- `git ls-tree origin/master` has `reclaim/`, `e2e/`, no `redis/` (origin/master `68dd267`)
- GitHub `get_me` as david-garcia-garcia; stub PR #3
- Research clone crowdsec-bouncer-traefik-plugin @ `6548da47` (deleted after write)
- PR comment list empty; check runs in progress on 34577992980

### Rank-up moves
None.
