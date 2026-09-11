Developer review: ready for review — 2026-09-11T05:41:52Z

## What this changes

**Operators.** None.

**Admin users.** None.

**Developers.** Import `github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim` for the keyed create/sleep/wake/close table. Prove Yaegi with `./Test-Integration.ps1` (fake plugin `e2e/reclaimprobe`, Traefik v3.7.11). CI runs lint, `go test`, and that Pester harness on PRs; a `v*` tag cuts a GoReleaser source release. Specs: `std_go_reclaim_context-lease`, `std_go_reclaim_value-lifecycle`.

**End users.** None.

## Motivation

Traefik middlewares need one shared value per key while any plugin instance holds it, and a cheap sleep window after the last holder drops. That table lived only in `traefik-geoblock` PR #83.

On `origin/initial` this repo had no module and no GitHub Actions. Compiled `go test` cannot see interpreter-only failures, and a PR with no workflows reports nothing.

If we do not land this library with a Traefik local-plugin e2e and the geoblock-style CI jobs, other middleware repos keep copying reclaim and this PR would look mergeable with no checks.

```mermaid
sequenceDiagram
  participant T as Traefik New
  participant P as reclaimprobe
  participant R as reclaim.Open
  T->>P: New /a and /b
  P->>R: Open key=shared
  Note over R: one put, two binds under Yaegi
```

## Merge readiness

Remote PR open. Lint, Test, and Integration succeeded. 1 follow-up remains.

Priority: P3 — spec, docs, tests, and internal library extraction.
Reviewed head: 1554325
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | CI succeeded; no open PR comments |
| CI proof | 6/6 | Lint, Test, Integration succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34566784096 |
| Local tests proof | N/A | prHost github; CI covers it |
| Review resolution | 6/6 | No OPEN reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-reclaim-table pushed | `git` / GitHub |
| OpenSpec | add-reclaim-table archived | `openspec/changes/archive/2026-09-11-add-reclaim-table/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/1 | GitHub MCP |
| CI | build 34566784096 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34566784096 | GitHub check runs |
| Local tests | passed | handoff.yaml |
| PR comments | no comments | comments none |

## Specs
- [std_go_reclaim_context-lease](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/openspec/changes/archive/2026-09-11-add-reclaim-table/proposal.md) — added
- [std_go_reclaim_value-lifecycle](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/openspec/changes/archive/2026-09-11-add-reclaim-table/proposal.md) — added

## Deviations from the ask
None.

## Follow-up issues
- [ ] [Yaegi drops the method set of a value returned as `any`](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md) — Yaegi strips methods on an `any` return, so optional reclaim lifecycle hooks never run interpreted.

## How this fits together
Local ticket `devstate/2026/09/2026-09-11-reclaim-table/` on branch `2026-09-11-reclaim-table`; durable card is PR #1 summary; CI run 34566784096 succeeded.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Fake middleware shape? | unranked — row incomplete | assumed — nested `e2e/reclaimprobe` | explore |
| Geoblock harness reuse? | unranked — row incomplete | assumed — pattern only | explore |
| E2e host vs CI? | unranked — row incomplete | assumed — this host; runner shipped; no push | explore |
| Change `Open` for Yaegi hooks? | unranked — row incomplete | assumed — no | explore |
| Traefik image / useunsafe? | unranked — row incomplete | assumed — v3.7.11; false | explore |
| Go version? | unranked — row incomplete | assumed — 1.21 | explore |
| Rename `Reset`? | unranked — row incomplete | assumed — keep `Reset` | explore |

## Before merge
None.

## Findings
- [[P3] Name for the scope on test-only Reset](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/devstate/2026/09/2026-09-11-reclaim-table/codereview_standards.md) — skipped — keep `Reset` for spec/geoblock alignment. Path: `reclaim/default.go`. Reply none.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/devstate/2026/09/2026-09-11-reclaim-table/codereview_standards.md) — 1 total, 0 pending, 0 completed, 1 skipped
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/devstate/2026/09/2026-09-11-reclaim-table/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/devstate/2026/09/2026-09-11-reclaim-table/codereview_spec.md) — 2 total, 0 pending, 1 completed, 1 skipped
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/devstate/2026/09/2026-09-11-reclaim-table/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/devstate/2026/09/2026-09-11-reclaim-table/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/devstate/2026/09/2026-09-11-reclaim-table/codereview_dead.md) — 3 total, 0 pending, 0 completed, 3 skipped
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-reclaim-table/devstate/2026/09/2026-09-11-reclaim-table/codereview_coverage.md) — 0 total, 0 pending, 0 completed

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 2 added / 0 modified | Archived into `openspec/specs/` |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | No comments.md |
| Reviewed head | 1554325644b443b1cd99f50eb662baaeada3dfd1 | CI ran on this SHA |

### Stored data model
None.

### Technical review
Best possible solution: Port geoblock's stdlib reclaim table into this library module, prove Traefik Yaegi with a nested fake plugin, and run the same lint / unit / Pester / tag-release jobs geoblock uses.

Do we have a high-confidence way to reproduce? Yes. `go test ./reclaim/...`, `./Test-Integration.ps1` (shared `X-Reclaim-ID=1`, one `reclaim_put` two `reclaim_bind`), and GitHub Actions run 34566784096.

Is this the best way to solve the issue? Yes.

### Evidence
What I checked:
- `go test ./reclaim/...` passed (local)
- `Test-Integration.ps1` 3/3 passed (local)
- GitHub Actions Lint / Test / Integration succeeded on `1554325` (run 34566784096)
- Catalog: `openspec/specs/std_go_reclaim_context-lease`, `std_go_reclaim_value-lifecycle`

### Rank-up moves
- In-process Yaegi interp module outside this `go.mod` as a faster CI guard than Docker.
