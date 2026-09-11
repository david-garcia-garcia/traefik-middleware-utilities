Developer review: ready for review — 2026-09-11T09:09:28Z

## What this changes
**Operators.** E2e compose runs `redis:7-alpine` at `redis:6379` (no password) plus `whoami-redis` on `/redis` on the existing `reclaim-e2e` stack (ports 8000/8080 unchanged).

**Admin users.** None.

**Developers.** Package `simpleredis/` is a copied stdlib RESP client (`Init`/`Get`/`MGet`/`Set`/`Del`/`Close`) with copied fake-TCP tests, `simpleredis/yaegi_test.go`, nested plugin `e2e/simpleredisprobe`, usage packet `std_go_simpleredis`, and catalog specs `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands`. Import `github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis`.

**End users.** None.

## Motivation
DestBranch listed Redis as Planned and had no client. Middleware authors could not import SimpleRedis from this module.

If this PR does not land, each plugin keeps inventing a RESP dialer, and there is no Yaegi or Traefik e2e seam for that client.

```mermaid
flowchart LR
  dest["DestBranch: Redis Planned, no tree"] --> copy["simpleredis/ copy"]
  copy --> yaegi["yaegi_test.go"]
  copy --> probe["e2e/simpleredisprobe + compose redis"]
  dest --> miss["Without merge: each plugin keeps its own dialer"]
```

## Merge readiness
OpenSpec archived, CI green, no open review comments. 0 items remain.

Priority: P3 — missing planned library; no production harm today
Reviewed head: 5572ad8
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Lint, Test, and Integration Tests succeeded |
| CI proof | 6/6 | Lint, Test, Integration Tests success — https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34582611526 |
| Local tests proof | N/A | `localTests: passed`; remote CI is the proof axis |
| Review resolution | 6/6 | OPEN PR #3, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-simpleredis pushed | `git` / origin/2026-09-11-simpleredis @ 5572ad8 |
| OpenSpec | add-simpleredis archived | `openspec/changes/archive/2026-09-11-add-simpleredis/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/3 | GitHub MCP |
| CI | build 34582611526 success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34582611526 | GitHub check runs |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | comments: none |

## Specs
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/openspec/changes/archive/2026-09-11-add-simpleredis/proposal.md) — added
- [std_go_simpleredis_resp-commands](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/openspec/changes/archive/2026-09-11-add-simpleredis/proposal.md) — added

## Deviations from the ask
- taken: README Planned Redis connection under `redis/` → folder and import `simpleredis/`, Libraries title SimpleRedis — `README.md` — dest sibling `reclaim/` kept folder=package. Requester: not asked.

## Follow-up issues
- [ ] [Rename compose project `reclaim-e2e` to a harness name](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/knowledge/debt/2026-09-11-rename-reclaim-e2e-compose.md) — compose project `reclaim-e2e` will also host the Redis probe after this change.
- [ ] [Choose a product LICENSE for dest](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/knowledge/debt/2026-09-11-choose-product-license.md) — dest has no root LICENSE while this change adds Apache-2.0 files under `simpleredis/`.

## How this fits together
Local ticket 2026-09-11-simpleredis is the branch and PR #3 into master. SimpleRedis is copied, proved under Yaegi and Traefik Pester, usage-documented, and archived.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Folder and import last segment — README `redis/` vs source package `simpleredis`? | additive asked | assumed — `simpleredis/` and `package simpleredis`. | explore |
| Ticket names simpleredis vs README “Redis connection” under `redis/`? | additive asked | assumed — Libraries title SimpleRedis; Status Current. | explore |
| Does the host plugin share the reclaim compose project or get its own stack/ports? | additive asked | assumed — share `reclaim-e2e`. | explore |
| Is e2e Redis a compose `redis` container or a fake like `startFakeRedis`? | additive asked | assumed — compose `redis:7-alpine` at `redis:6379`. | explore |
| How does Apache-2.0 attribution land given dest has no root LICENSE? | additive incidental | assumed — package-local `simpleredis/LICENSE`. | explore |
| Does `Test-Integration.ps1` stay one script for both libraries or split? | additive asked | assumed — one runner and one Pester file. | explore |
| What is the fake middleware shape for Redis Yaegi e2e? | additive asked | assumed — nested `e2e/simpleredisprobe`. | explore |
| Where do Yaegi interpreter tests live? | additive asked | assumed — `simpleredis/yaegi_test.go`. | explore |

## Before merge
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/devstate/2026/09/2026-09-11-simpleredis/codereview_standards.md) — 4 total, 0 pending, 4 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/devstate/2026/09/2026-09-11-simpleredis/codereview_nitpicks.md) — 3 total, 0 pending, 1 completed, 2 skipped
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/devstate/2026/09/2026-09-11-simpleredis/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/devstate/2026/09/2026-09-11-simpleredis/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/devstate/2026/09/2026-09-11-simpleredis/codereview_performance.md) — 1 total, 0 pending, 0 completed, 1 skipped
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/devstate/2026/09/2026-09-11-simpleredis/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis/devstate/2026/09/2026-09-11-simpleredis/codereview_coverage.md) — 5 total, 0 pending, 4 completed, 1 skipped

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 2 added / 0 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 5572ad899863170473381430a561fb781bab3073 | Card must match the branch you measured |

### Stored data model
- New: Redis keys written by `e2e/simpleredisprobe` (probe SET/GET `simpleredisprobe`).

### Technical review
Best possible solution: Copy the named stdlib client as `simpleredis/` and prove it with dest’s Yaegi + shared-compose Pester pattern.

Do we have a high-confidence way to reproduce? Yes — `go test ./simpleredis/...` and CI Integration Tests on 34582611526.

Is this the best way to solve the issue? Yes versus DestBranch.

### Evidence
What I checked:
- `openspec/changes/archive/2026-09-11-add-simpleredis/`
- `knowledge/devdocs/std_go_simpleredis.md`
- GitHub check runs 34582611526: Lint/Test/Integration success
- comments: none

### Rank-up moves
None.
