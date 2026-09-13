Developer review: in progress — 2026-09-13T15:27:45Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None. Explore measured dest lint; enablement is later phases.

**End users.** None.

## Motivation
Dest lint is a ten-linter `.golangci.yml` and CI `version: latest`. Named results, helper `t.Helper()`, identity error compares that the SimpleRedis retry spec requires, and Yaegi `Eval` type asserts are unguarded. golangci-lint v2 would also reject this config schema.

Re-measure on dest `2847a81` with v1.63.4 (uncapped): the 17 listed linters are genuinely zero; gocritic with `unnamedResult` added is 20 hits (17 unnamedResult + 3 default `assignOp`); errorlint is 10 `==` sites (the type-assert site is gone); testpackage is 56 and stays off.

Cost of not merging: later PRs can land unnamed results and `errors.Is` retries of `ErrPoolWait`, and CI can jump to a v2 linter that does not read this file.

## Merge readiness
Explore measured dest hits and ranked the open questions. Product lint enablement is not in this diff. 1 item remains.

Priority: P3 — spec, docs, tests, or internal clarity, no current user or operator harm
Reviewed head: 7b2d87a
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Stub CI succeeded; product work not started |
| CI proof | 6/6 | run 34765231667 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34765231667 |
| Local tests proof | N/A | remote PR; CI is the proof axis |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-golangci-lint-harden pushed | git / origin 7b2d87a |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/72 | pr-host |
| CI | build 34765231667 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34765231667 | all eight jobs success |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket is local spec → branch `2026-09-13-golangci-lint-harden` off `origin/master` → PR 72. Dest lint re-measured on `2847a81` with v1.63.4. Enablement is propose/implement. Merge after in-flight SimpleRedis branches that touch `borrow` / `dial` and identity `==` sites: `2026-09-13-simpleredis-panic-safe-release`, `2026-09-13-simpleredis-lost-turn-recovery`, `2026-09-13-simpleredis-desync-boundary-check`, `2026-09-13-simpleredis-close-abandoned-socket`, `2026-09-13-simpleredis-resilience-test-coverage`.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Fold the lint pin into `std_go_ci_test-suites`, or add a new spec leaf? | additive asked | assumed — fold the pin + enabled-linter contract into `std_go_ci_test-suites`; usage gotchas into `std_go_test-suites.md`. Propose runs FindSpecHost. | explore |

## Before merge
- [ ] Land Tasks 1–6 with no runtime behavior change (named results, `t.Helper()`, documented `errorlint` nolint, pin `v1.63.4`)
- [x] Re-measure linters on dest master with v1.63.4 uncapped
- [x] Stub PR 72 opened from `origin/master`
- [x] Stub CI green (run 34765231667)

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
| Reviewed head | 7b2d87af5aff837ef8dbfcb5a81f537b34ab87c3 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest lint is config-only plus mechanical fixes; this phase measured dest hits and did not change product code.

Do we have a high-confidence way to reproduce? Yes — `golangci-lint` v1.63.4 on dest `2847a81` with uncapped issues. Counts are in `explore.md`.

Is this the best way to solve the issue? Yes vs dest: enable and pin the linter, then fix only what it demands. Convert none of the ten errorlint `==` sites.

### Evidence
What I checked:
- 17 listed linters: 0 hits on dest
- gocritic + `unnamedResult`: 20 (17 unnamedResult + 3 assignOp); `checkExported: true` drops unnamedResult to 3
- `borrow` / `dial` not flagged by unnamedResult
- errorlint 10; handshakeFailure type assert `not found`
- thelper 15, revive 8, forcetypeassert 14 (all `_test.go`), dupword 1, prealloc 1, stylecheck 0, errname 0, testpackage 56
- goimports: `exec: "diff": executable file not found` on this Windows host
- CI run 34765231667: eight jobs success

### Rank-up moves
None.

[sgsi-dev-ticket-status:2026-09-13-golangci-lint-harden]
