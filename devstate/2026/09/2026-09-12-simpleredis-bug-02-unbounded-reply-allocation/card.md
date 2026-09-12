Developer review: in progress — 2026-09-12T12:41:57Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Notes a deferred cumulative array-reply budget at `knowledge/debt/2026-09-12-simpleredis-cumulative-array-reply-budget.md`. No `$`/`*` decode ceiling has landed yet.

**End users.** None.

## Motivation
On DestBranch, SimpleRedis sizes a heap slice from the `$` or `*` length in the reply header before any payload byte arrives. Lengths that fit in `int` are honoured. A 12-byte `$268435456` header allocates 256 MiB. Larger lengths panic in `make`. That panic is not this ticket’s pool-leak fix; the quieter case is the process-wide OOM.

The truncated ReadFull after that allocate is EOF, mapped to `redis:unreachable`, which retries (default three extra attempts). One Get can repeat a 256 MiB allocate. Wrong-port HTTP, a desynced socket, or a hostile Redis all reach the same parse. If we do not merge, that path stays a remote-triggerable Traefik process kill.

```mermaid
sequenceDiagram
  participant Peer
  participant Decoder as readBulk
  participant Heap
  participant Retry as shouldRetry
  Peer->>Decoder: 12-byte header $268435456
  Decoder->>Heap: make 256 MiB
  Decoder-->>Retry: EOF as redis:unreachable
  Retry->>Decoder: up to 3 extra attempts
```

## Merge readiness
Explore recorded (`qualified-with-gaps`). Caps and tests are not applied. 2 items remain.

Priority: P1 — a remote peer can size a heap allocation large enough to kill the Traefik process today
Reviewed head: 77b8a42
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI still in progress on the stub PR |
| CI proof | 3/6 | Lint, Test, Go E2E, Integration Tests queued — [run 34694330457](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34694330457) |
| Local tests proof | N/A | Before implement; remote CI is the proof axis |
| Review resolution | 6/6 | OPEN PR 34; no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-12-simpleredis-bug-02-unbounded-reply-allocation pushed | `git` / GitHub |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/34 | pr-host |
| CI | build 34694330457 queued https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34694330457 | GitHub check runs |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | no `comments.md` |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
- [ ] [Cumulative array-reply decode budget](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-12-simpleredis-bug-02-unbounded-reply-allocation/knowledge/debt/2026-09-12-simpleredis-cumulative-array-reply-budget.md) — per-element and count caps still allow a huge total array reply.

## How this fits together
Local ticket on branch `2026-09-12-simpleredis-bug-02-unbounded-reply-allocation` opened PR 34 against `master`. Explore kept the ticket caps; apply has not started.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| How to spell MaxInt64 / panic-length cases on 32-bit `int`? | additive asked | assumed — feed header digits from `strconv.FormatInt(math.MaxInt64, 10)`. On 32-bit, `parseLen` already returns false above `maxParseLen` (`errIssue`, `clean == false`) before the new cap. On 64-bit, `MaxInt` would panic in `make([]byte, length+2)` today (`length+2` wraps); the cap runs first. Do not skip the test. Do not pass `int(MaxInt64)` as a length on 32-bit. | explore |
| Who already owns client identity (address, user, tenant, Host, trust hop) for this change? | additive incidental | assumed — none. The cap classifies a RESP header; it does not set or rebuild a host fact. | explore |

## Before merge
- [ ] Cap `$` and `*` reply allocations at 64 MiB / `1 << 20` and return `redis:issue?` without the `make`
- [ ] Prove over-cap, panic-length, and `$268435456` allocation with `readReply` tests
- [x] Stub PR 34 opened
- [x] Explore recorded package consts and test spelling

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | No spec.md vs `master` yet |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 77b8a426a9a767aac2e52245cda5f5609b11ce26 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: package consts in front of `make` so over-cap headers are `redis:issue?` and not retried, matching DestBranch error classes.

Do we have a high-confidence way to reproduce? Yes — `readBulk` / `readReply` in `simpleredis/resp.go` plus `shouldRetry` on `errUnreachable`; no over-cap test exists today.

Is this the best way to solve the issue? Yes — ticket-named consts, not Config, keep `make([]byte, length+2)` for accepted lengths.

### Evidence
What I checked:
- `simpleredis/resp.go` `readBulk` `make([]byte, length+2)`, `readReply` `*` `make([][]byte, count)`, `parseLen` overflow, `ioError` (worktree at 77b8a42)
- `simpleredis/commands_exec.go` `shouldRetry` / `retryLimits` (3 extra retries)
- `simpleredis/resp_test.go` garbage length and truncated `$10`; no `$268435456` alloc guard
- `simpleredis/bench_test.go` `TestAllocDecodeBulk100KB` is 100 KB, under 64 MiB
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` requires `make([]byte, length+2)` + `ReadFull`
- PR 34; checks queued (run 34694330457)

### Rank-up moves
None.
