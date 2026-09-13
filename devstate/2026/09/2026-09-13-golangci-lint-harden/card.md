Developer review: needs changes — 2026-09-13T15:17:48Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Motivation
Dest lint is a ten-linter `.golangci.yml` and CI `version: latest`. Named results, helper `t.Helper()`, identity error compares that the SimpleRedis retry spec requires, and Yaegi `Eval` type asserts are unguarded. golangci-lint v2 would also reject this config schema.

Cost of not merging: later PRs can land unnamed results and `errors.Is` retries of `ErrPoolWait`, and CI can jump to a v2 linter that does not read this file.

## Merge readiness
Prepare grounded the ticket and opened the stub PR. Lint config is not in this diff. Integration Tests failed on the empty stub. 2 items remain.

Priority: P3 — spec, docs, tests, or internal clarity, no current user or operator harm
Reviewed head: 6b041d9
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 2/6 | CI failed on the empty stub |
| CI proof | 2/6 | run 34765058455 failed https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34765058455 |
| Local tests proof | N/A | remote PR; CI is the proof axis |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-golangci-lint-harden pushed | git / origin 6b041d9 |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/72 | pr-host |
| CI | build 34765058455 failed https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34765058455 | Integration Tests failed; Lint, Unit, Unit race, Go E2E Redis, Go E2E Dragonfly, Integration Tests Redis, Integration Tests Dragonfly succeeded |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket is local spec → branch `2026-09-13-golangci-lint-harden` off `origin/master` → PR 72. Empty start commit only. Lint enablement is later phases. Merge after in-flight SimpleRedis branches that touch `borrow` / `dial` and identity `==` sites: `2026-09-13-simpleredis-panic-safe-release`, `2026-09-13-simpleredis-lost-turn-recovery`, `2026-09-13-simpleredis-desync-boundary-check`, `2026-09-13-simpleredis-close-abandoned-socket`, `2026-09-13-simpleredis-resilience-test-coverage`.

## Explore Decisions
None.

## Before merge
- [ ] Re-measure linters on dest master and land Tasks 1–6 with no runtime behavior change
- [ ] [P3] Integration Tests failed on stub CI (run 34765058455) — dest or flake; do not merge until measured green
- [x] Stub PR 72 opened from `origin/master`
- [x] Ticket grounded (`qualified-with-gaps`: hit counts not re-measured this phase)

## Findings
- [[P3] Integration Tests failed on the empty stub](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34765058455) — self-found — product delta is the empty start commit; this is dest CI or flake, not a lint change. Path: (general).

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 6b041d9c4ebd9d2049b9ced77eb6da2361a45897 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest lint is config-only plus mechanical fixes; this phase grounded the ticket and opened the review PR, and did not change product code.

Do we have a high-confidence way to reproduce? No lint re-measure on dest this phase. Requester counts are from another branch.

Is this the best way to solve the issue? Yes vs dest: enable and pin the linter, then fix only what it demands.

### Evidence
What I checked:
- `.golangci.yml` ten enable entries, no issues block (`2847a81`)
- `.github/workflows/ci.yml` `version: latest`
- `simpleredis/pool.go` `borrow` / `dial` unnamed `(*pooledConn, error, bool)`; `handshakeFailure` type absent
- `tokenbucket/clock.go` three `tokens = tokens ±` assigns
- `simpleredis/commands_exec.go` `err == errTimeout` / `err == errUnreachable`; tcp-session spec requires identity for pool wait
- Yaegi `evaluated.Interface().(string)` in seven `_test.go` files
- No `package *_test` in this tree
- CI run 34765058455: Integration Tests failure; other seven jobs success

### Rank-up moves
None.
