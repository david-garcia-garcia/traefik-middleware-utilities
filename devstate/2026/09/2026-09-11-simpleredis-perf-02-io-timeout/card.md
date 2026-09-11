Developer review: ready for review — 2026-09-11T22:24:22Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** SimpleRedis adds Yaegi-safe `InitWithOptions` and `Options` (`DialTimeout`, `IoTimeout`, `IdleTimeout`, `MaxIdleConns`; zero means today's 2s / 1s / 30s / 8). Configured I/O timeout is proven with `BLPOP` against live Redis and Dragonfly; CI `Test` sets `SIMPLEREDIS_LIVE_REDIS` and `SIMPLEREDIS_LIVE_DRAGONFLY`. `Init(host, pass, database)` is unchanged. Live-socket wait queue stays with perf-01. Usage packet `std_go_simpleredis` names those knobs. Live spec `std_go_simpleredis_tcp-session` has the delta folded; change archived as `2026-09-11-simpleredis-configurable-timeouts`.

**End users.** None.

## Motivation
SimpleRedis on master uses compile-time deadlines: 1 second for each command I/O and 2 seconds for a new dial. Callers that sit in front of Redis or Dragonfly (rate limiters, cache) cannot ask for tens of milliseconds and a fast fallback.

When Redis stalls, each in-flight command holds its socket until that 1 second deadline. There is no wait queue, so every new request dials another socket. A degraded backend is exactly when the client opens the most connections, and Traefik workers block for that second. Leaving the constants in place keeps that fan-out as the only behaviour.

```mermaid
sequenceDiagram
  participant Caller
  participant Client as SimpleRedis
  participant Redis
  Caller->>Client: command
  Client->>Redis: write and read with 1s deadline
  Note over Redis: stall
  Caller->>Client: another command
  Client->>Redis: new TCP dial (no live-socket cap)
```

## Merge readiness
WIP dropped. One OPEN PR. CI succeeded. 0 items remain.

Priority: P2 — Redis slowness holds Traefik workers for a full second and fans out dials, with no caller-set shorter deadline
Reviewed head: 4f53486
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | CI succeeded; no open PR comments |
| CI proof | 6/6 | Lint, Test, and Integration Tests succeeded |
| Local tests proof | N/A | Remote CI covers this host |
| Review resolution | 6/6 | OPEN PR, no comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-simpleredis-perf-02-io-timeout pushed | `git` origin/2026-09-11-simpleredis-perf-02-io-timeout |
| OpenSpec | simpleredis-configurable-timeouts | `openspec/changes/archive/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/16 | pr-host List/Create |
| CI | build 34653707358 success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34653707358 | pr-host CI |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-02-io-timeout/openspec/changes/archive/2026-09-11-simpleredis-configurable-timeouts/proposal.md) — modified

## Deviations from the ask
- taken: finding listed poolSize/poolTimeout as config fields → Options has only DialTimeout, IoTimeout, IdleTimeout, MaxIdleConns — `simpleredis/simpleredis.go` — unused live-socket fields would lie until perf-01. Requester: not asked.

## Follow-up issues
None.

## How this fits together
Local finding perf-02 is on branch `2026-09-11-simpleredis-perf-02-io-timeout` from `master`. PR 16 is the durable card host, title ready, CI green.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Options surface — InitWithOptions, extra Init args, or exported fields on SimpleRedis? | additive asked | assumed — InitWithOptions(host, pass, database, Options) with unexported session fields from a plain Options; Init calls it with Options{}; not extra Init args; not exported timeout fields on SimpleRedis | explore |
| How to stall a live Redis and Dragonfly command to prove a configured I/O timeout without DEBUG SLEEP, Lua 5.1-safe, KEYS-declared? | additive asked | assumed — same-package live test exec of BLPOP on a unique empty key with server timeout 10s and IoTimeout ~50ms; not Eval, CLIENT PAUSE, DEBUG SLEEP, or a public BLPOP method | explore |
| Whether unused poolSize/poolTimeout fields are stored as no-ops or omitted from the struct and only documented? | additive asked | assumed — omit from Options; document live-socket cap / wait queue as perf-01 | explore |
| Must the Traefik probe Config expose the new timeouts for Pester, or are live Go tests against compose/CI ports enough? | additive asked | assumed — live Go tests are enough; probe stays Host only; CI test job sets SIMPLEREDIS_LIVE_REDIS and SIMPLEREDIS_LIVE_DRAGONFLY | explore |
| How is configured DialTimeout proven if a blackhole SYN is not reliable in CI? | additive asked | assumed — package test that dial uses the stored duration plus hanging-SYN to 192.0.2.1 asserting redis:unreachable well under the 2s default; I/O timeout is the live-engine proof | explore |
| Can MaxIdleConns 0 mean "keep no idle sockets", or is 0 the default eight? | additive asked | assumed — MaxIdleConns <= 0 means eight; callers cannot express idle cap zero on this Options type | explore |
| Rewrite the spec's "concurrent commands SHALL not open more than eight connections" now that MaxIdleConns is configurable? | bounded incidental | assumed — do not rewrite the live-socket claim; spec delta idle list at most MaxIdleConns (default 8); leave the live cap to perf-01 | explore |

## Before merge
- [x] [P2] Make dial, I/O, idle timeout, and idle cap configurable with today's numbers as defaults
- [x] [P2] Prove the new knobs on live Redis and Dragonfly (fake-server is not enough)
- [x] Keep poolSize/poolTimeout as knobs or documented tension; do not ship perf-01's wait-queue semaphore

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-02-io-timeout/devstate/2026/09/2026-09-11-simpleredis-perf-02-io-timeout/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-02-io-timeout/devstate/2026/09/2026-09-11-simpleredis-perf-02-io-timeout/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-02-io-timeout/devstate/2026/09/2026-09-11-simpleredis-perf-02-io-timeout/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-02-io-timeout/devstate/2026/09/2026-09-11-simpleredis-perf-02-io-timeout/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-02-io-timeout/devstate/2026/09/2026-09-11-simpleredis-perf-02-io-timeout/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-02-io-timeout/devstate/2026/09/2026-09-11-simpleredis-perf-02-io-timeout/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-perf-02-io-timeout/devstate/2026/09/2026-09-11-simpleredis-perf-02-io-timeout/codereview_coverage.md) — 1 total, 0 pending, 1 completed

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 4f534869fbdaae52aaa1b9d7cca8fc506c96d4b9 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Keep Init(host, pass, database) and today's defaults; add InitWithOptions with a plain Options struct; prove I/O timeout with BLPOP on both engines; leave the wait-queue to perf-01.

Do we have a high-confidence way to reproduce? Yes — TestIoTimeout on defaults; configured IoTimeout fake never-reply; live BLPOP on Redis and Dragonfly; hanging-SYN for DialTimeout; TestInitStoresDefaultTimeouts for zero/negative defaults.

Is this the best way to solve the issue? Yes versus master: expose the existing constants as caller-set deadlines without rewriting the pool.

### Evidence
What I checked:
- PR 16 title ⚡️ perf(simpleredis): expose caller-set dial and I/O timeouts, state open
- HEAD 4f53486 pushed
- OPEN PR 16, zero comments
- CI run 34653707358: Lint success, Test success, Integration Tests success

### Rank-up moves
None.
