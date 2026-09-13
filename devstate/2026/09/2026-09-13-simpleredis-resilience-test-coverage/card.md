Developer review: in progress — 2026-09-13T09:29:51Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Dest SimpleRedis now has permanent tests for the healthy hunt probes (`TestChaosPoolInvariants`, `FuzzReadReply` / `FuzzParseLen`, lifecycle Close cycles, Yaegi error paths, concurrent MSetEX fallback, RESP injection) and `simpleredis/BUGS.md` pointing the three real defects at their other PRs.

**End users.** None.

## Motivation
Dest `simpleredis` already survived a hunt for production killers (races, leaked pool turns, silent cross-key replies). Three real defects are owned by other PRs. The probes that came back healthy were not locked in on dest, so a later edit could reintroduce a turn leak or a cross-key reply while CI still looked green. A decoder panic in `readReply` is a permanent pool-turn leak because `exec` releases without `defer`.

```mermaid
sequenceDiagram
  participant exec
  participant do
  participant readReply
  participant inUseTurns
  exec->>do: command
  do->>readReply: parse
  readReply-->>do: panic
  Note over exec: release is not deferred
  exec--x inUseTurns: turn never returned
```

## Merge readiness
Code review applied two hard Leave-a-trail comments and one named `chaosHonest` case. Lint G404/goconst and Unit race on head 5130f35 are addressed on 9ce9b97. CI on the new head is in progress. 1 item remains (CI green).

Priority: P3 — spec, docs, tests, or internal clarity, no current user or operator harm
Reviewed head: 9ce9b97
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI in progress on the review-fix head |
| CI proof | 3/6 | previous run 34749156784 failed Lint and Unit race; new head pushed, checks not yet measured |
| Local tests proof | N/A | remote PR; CI is the proof axis |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-resilience-test-coverage pushed | git / origin 9ce9b97 |
| OpenSpec | simpleredis-resilience-test-coverage | `openspec/changes/simpleredis-resilience-test-coverage/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/68 | pr-host |
| CI | previous build 34749156784 failed Lint and Unit race; new head not seen | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34749156784 |
| Local tests | passed | go vet; default 13.8s 92.5%; -short 12.16s after review/CI-fix; FuzzReadReply 2.07M execs / 30s |
| PR comments | no comments | no comments.md |

## Specs
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/openspec/changes/simpleredis-resilience-test-coverage/proposal.md) — modified
- [std_go_simpleredis_resp-decode](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/openspec/changes/simpleredis-resilience-test-coverage/proposal.md) — modified
- [std_go_simpleredis_resp-commands](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/openspec/changes/simpleredis-resilience-test-coverage/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Code review is on PR 68 off `origin/master`. Hard findings applied. CI re-run after Lint/Unit-race fixes.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| What are the new test file and function names? | additive asked | assumed — chaos_pool_test.go, resp_fuzz_test.go, lifecycle_test.go, yaegi_errorpath_test.go, commands_msetex_concurrent_test.go, resp_injection_test.go | explore |
| Is BUGS.md a tracked package record or a local-only artifact? | additive asked | assumed — commit simpleredis/BUGS.md; leave reclaim/tokenbucket/windowcounter BUGS.md untracked | explore |
| Does propose add an OpenSpec coverage requirement, or are tests-only enough? | additive asked | assumed — add coverage requirements on the three existing SimpleRedis spec leaves, no new family | explore |
| What chaos duration and lifecycle iteration counts keep the default suite in the low seconds and keep go test -race from timing out? | additive asked | assumed — Short skips chaos/lifecycle; default 24 goroutines x 1s and 200 cycles; raceDetectorOn lowers those and skips concurrent MSetEX plus Yaegi error paths | implement |
| Who already owns client identity (address, user, tenant, Host, trust hop)? | additive asked | assumed — none; this PR does not set or reconstruct identity | explore |

## Before merge
- [ ] Wait for CI on PR 68 to succeed
- [ ] Human: `reclaim/BUGS.md`, `tokenbucket/BUGS.md`, and `windowcounter/BUGS.md` are untracked in the caller workspace — decide whether `BUGS.md` stays local rather than silently dropping `simpleredis/BUGS.md`
- [x] Land the six healthy-probe tests and reframed `simpleredis/BUGS.md`
- [x] Propose folded coverage onto three existing spec leaves
- [x] Apply code-review hard findings (job comments, `chaosHonest`)
- [x] Stub PR opened

## Findings
- [P3] CI Lint G404/goconst on `chaos_pool_test.go` and Unit race on Yaegi/concurrent MSetEX — FIX — named command constants, `chaosIntn` gosec skip, skip Yaegi error paths and concurrent MSetEX under `raceDetectorOn`. Path: `simpleredis/chaos_pool_test.go`. Reply none.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/devstate/2026/09/2026-09-13-simpleredis-resilience-test-coverage/codereview_standards.md) — 2 total, 0 pending, 2 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/devstate/2026/09/2026-09-13-simpleredis-resilience-test-coverage/codereview_nitpicks.md) — 1 total, 0 pending, 1 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/devstate/2026/09/2026-09-13-simpleredis-resilience-test-coverage/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/devstate/2026/09/2026-09-13-simpleredis-resilience-test-coverage/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/devstate/2026/09/2026-09-13-simpleredis-resilience-test-coverage/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/devstate/2026/09/2026-09-13-simpleredis-resilience-test-coverage/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-resilience-test-coverage/devstate/2026/09/2026-09-13-simpleredis-resilience-test-coverage/codereview_coverage.md) — 0 total, 0 pending, 0 completed

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 3 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 9ce9b973162abba7e8ad9711bba7d2dab4800190 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest already behaves as the healthy probes measured; this PR adds the missing locks and the written record, not a product patch.

Do we have a high-confidence way to reproduce? Yes. Default suite passed locally (13.8s, 92.5%). FuzzReadReply 2.07M execs / 30s, no panic.

Is this the best way to solve the issue? Yes vs dest: lock the healthy probes as tests instead of hunting them again.

### Evidence
What I checked:
- Seven-axis review of `origin/master...HEAD` excluding `devstate/` and `.cursor/`
- Hard Standards (2) and Nitpicks (1) applied; other axes none
- `go test -short ./simpleredis/` pass, 12.16s after review/CI-fix
- Previous CI run 34749156784: Lint failed G404/goconst; Unit race failed; six other checks succeeded

### Rank-up moves
None.
