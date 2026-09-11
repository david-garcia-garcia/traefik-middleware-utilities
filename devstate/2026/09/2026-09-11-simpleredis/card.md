Developer review: in progress — 2026-09-11T08:36:49Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `add-simpleredis` specs the copy of SimpleRedis as `simpleredis/` (TCP session + RESP commands), Yaegi tests, and `e2e/simpleredisprobe` on the existing reclaim compose with Redis; library sources are not applied yet.

**End users.** None.

## Motivation
DestBranch still has no Redis client. Middleware authors cannot import SimpleRedis from this module. Propose now records the copy, Yaegi proof, and shared-compose Redis harness so apply has a delta instead of inventing a second client.

If this PR does not land, each plugin keeps a private RESP dialer and the Planned library never gets a Traefik e2e seam.

```mermaid
flowchart LR
  dest["DestBranch: Redis Planned"] --> spec["add-simpleredis specs"]
  spec --> apply["Copy + Yaegi + Pester still unapplied"]
  dest --> miss["Without merge: each plugin keeps its own dialer"]
```

## Merge readiness
Propose wrote apply-ready artifacts. Apply has not started. 1 item remains.

Priority: P3 — missing planned library; no production harm today
Reviewed head: 9d05aee
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI in progress; library not applied |
| CI proof | 3/6 | Lint, Test, Integration Tests in progress — https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34580010980 |
| Local tests proof | N/A | `localTests: none`; remote CI is the proof axis |
| Review resolution | 6/6 | OPEN PR #3, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-simpleredis pushed | `git` / origin/2026-09-11-simpleredis @ 9d05aee |
| OpenSpec | add-simpleredis | `openspec/changes/add-simpleredis/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/3 | GitHub MCP |
| CI | build 34580010980 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34580010980 | GitHub check runs |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | comments: none |

## Specs
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/openspec/changes/add-simpleredis/proposal.md) — added
- [std_go_simpleredis_resp-commands](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/openspec/changes/add-simpleredis/proposal.md) — added

## Deviations from the ask
- taken: README Planned Redis connection under `redis/` → folder and import `simpleredis/`, Libraries title SimpleRedis — `README.md` — dest sibling `reclaim/` kept folder=package; renaming the copied package would add a variation. Requester: not asked.

## Follow-up issues
- [ ] [Rename compose project `reclaim-e2e` to a harness name](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/knowledge/debt/2026-09-11-rename-reclaim-e2e-compose.md) — compose project `reclaim-e2e` will also host the Redis probe after this change.
- [ ] [Choose a product LICENSE for dest](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/knowledge/debt/2026-09-11-choose-product-license.md) — dest has no root LICENSE while this change adds Apache-2.0 files under `simpleredis/`.

## How this fits together
Local ticket 2026-09-11-simpleredis is the branch and stub PR into master. Propose added `add-simpleredis`; implement still has to copy the library and prove it under Yaegi.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Folder and import last segment — README `redis/` vs source package `simpleredis`? | additive asked | assumed — `simpleredis/` and `package simpleredis`. README Layout `redis/` becomes `simpleredis/`. Do not change the package clause to `redis`. | explore |
| Ticket names simpleredis vs README “Redis connection” under `redis/`? | additive asked | assumed — Libraries title SimpleRedis (exported type); Role is the shared stdlib RESP client; Status Current. | explore |
| Does the host plugin share the reclaim compose project or get its own stack/ports? | additive asked | assumed — share `docker-compose.yml` project `reclaim-e2e`. Add redis + `simpleredisprobe` + a whoami that is not a/b. | explore |
| Is e2e Redis a compose `redis` container or a fake like `startFakeRedis`? | additive asked | assumed — Pester uses compose `redis:7-alpine` at `redis:6379` with no password and empty database. | explore |
| How does Apache-2.0 attribution land given dest has no root LICENSE? | additive incidental | assumed — package-local `simpleredis/LICENSE`. Do not add a repo-root LICENSE. | explore |
| Does `Test-Integration.ps1` stay one script for both libraries or split? | additive asked | assumed — one `Test-Integration.ps1` and one `scripts/integration-tests.Tests.ps1`. | explore |
| What is the fake middleware shape for Redis Yaegi e2e? | additive asked | assumed — nested module `e2e/simpleredisprobe`; New Inits; handler SET+GET plus a response header. | explore |
| Where do Yaegi interpreter tests live? | additive asked | assumed — `simpleredis/yaegi_test.go`. | explore |

## Before merge
- [ ] Land the copied SimpleRedis library, Yaegi tests, Redis host plugin, and Pester e2e [P3]
- [x] Propose OpenSpec `add-simpleredis` (`std_go_simpleredis_tcp-session`, `std_go_simpleredis_resp-commands`)
- [x] Decide `simpleredis/` vs `redis/` and how e2e Redis is provided
- [x] Stub PR #3 opened into master

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 2 added / 0 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 9d05aee21aa0ac8828be79206890a66f7dee21ac | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Spec the copied stdlib client as two dest-shaped leaves (TCP session vs RESP commands) rather than a `redis/` rename DestBranch never shipped.

Do we have a high-confidence way to reproduce? Yes — dest has no `simpleredis/`; `openspec/changes/add-simpleredis/` is apply-ready.

Is this the best way to solve the issue? Yes versus DestBranch: two new `std_go_simpleredis_*` leaves, not a fold into reclaim.

### Evidence
What I checked:
- `openspec/changes/add-simpleredis/` proposal, design, tasks, two spec deltas (HEAD 9d05aee)
- `devstate/specs.md` FindSpecHost both `new`
- GitHub check runs on 34580010980 in progress
- comments: none

### Rank-up moves
None.
