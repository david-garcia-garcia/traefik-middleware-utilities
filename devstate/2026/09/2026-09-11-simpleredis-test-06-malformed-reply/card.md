Developer review: in progress — 2026-09-11T21:31:24Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Explore keeps RESP2 `*-1` as `redis:issue?` (comment at `count < 0`); malformed coverage is a fake-server table plus write-then-close truncations; live `/redis` and `/dragonfly` stay and gain Get-miss. Versus `master` this branch still only adds `knowledge/research/ext_redis_resp_null-array/` — no decoder or test code yet.

**End users.** None.

## Motivation
On `master`, `readReply` already fails empty lines, unknown type bytes, bad array counts (including `*-1`), missing CR, and truncated array elements as `redis:issue?` and marks the socket dirty. Almost none of those branches run. Only nested-array and garbage-integer cases exist, and they do not assert the idle list is empty. A proxy, TLS-as-plaintext, or HTTP on the Redis port is how those paths actually fire.

If we do not merge the coverage, a later decoder change can ship with those failure modes unproven, and a reused poisoned connection can contaminate the next request. Live `/redis` and `/dragonfly` already prove the happy path; they do not prove the garbage path. Explore closed the open `*-1` choice: keep it as a protocol violation, not `redis:miss`.

```mermaid
sequenceDiagram
  participant Cmd as command
  participant Reply as readReply
  participant Idle as idle pool
  Cmd->>Reply: canned or live reply
  alt malformed or unknown type
    Reply-->>Cmd: redis:issue? or I/O
    Reply->>Idle: socket stays out
  else RESP2 null array *-1
    Note over Reply: keep as protocol violation
    Reply-->>Cmd: redis:issue?
    Reply->>Idle: socket destroyed
  else live Redis or Dragonfly happy path
    Reply-->>Cmd: verb succeeds
  end
```

## Merge readiness
Explore closed `*-1` and the test shape; stub PR is open; CI is still running. 3 items remain before merge.

Priority: P3 — tests and decoder proof, no current operator or user harm
Reviewed head: 059fe5b
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI still in progress; explore only |
| CI proof | 3/6 | Test, Integration Tests, and Lint in progress |
| Local tests proof | N/A | Before implement; remote CI is the proof axis |
| Review resolution | 6/6 | No PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-simpleredis-test-06-malformed-reply pushed | `git` / origin |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/24 | pr-host List/Create |
| CI | build 34649731758 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34649731758 | pr-host CI |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
None.

## Deviations from the ask
- taken: uniform `idle==0` on `Get` / `parseIntegerReply` arity mismatch → `idle==0` only on dirty decoder rows — `simpleredis/simpleredis.go` — those checks run after a clean `readReply`. Requester: not asked.

## Follow-up issues
None.

## How this fits together
Local test-06 finding → explore on `2026-09-11-simpleredis-test-06-malformed-reply` → PR 24 → CI run 34649731758 still in progress.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| How to exercise `exec` borrow-failure on the retry attempt (`:195`) with a canned reply? | additive asked | assumed — dedicated one-accept helper (good first reply, then malformed on the reused socket, listener closed before retry dial). Expect `redis:unreachable` and `idle==0`. | explore |
| Do `Get` / `parseIntegerReply` count-mismatch rows assert `len(idle)==0` the same as decoder poison? | additive asked | assumed — cover with canned `*0` / `*2`; assert `redis:issue?`; do not assert `idle==0`. | explore |
| How do live Redis and Dragonfly stay in the proof if malformed cases are fake-server only? | additive asked | assumed — keep compose both engines and both whoami routes; extend probe + Pester with Get-miss on `/redis` and `/dragonfly`. | explore |
| Which spec leaf takes malformed and null-array? | additive asked | assumed — delta on `std_go_simpleredis_resp-commands`. No new spec family. | explore |

## Before merge
- [x] Decide RESP2 null-array (`*-1`) on explore.md — keep as `redis:issue?` + comment at `count < 0`
- [ ] [P3] Table-driven fake-server malformed replies with `len(idle) == 0` on dirty rows
- [ ] [P3] Keep live `/redis` and `/dragonfly` happy-path; extend Get-miss on both engines

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
| Reviewed head | 059fe5be24f3c7813bd6ca9dde705a4f0d8eba1b | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: keep `*-1` as issue (no verb receives a null array), prove poison paths with a fake-server table, and keep live Redis and Dragonfly so a decoder change cannot ship unproven.

Do we have a high-confidence way to reproduce? Yes, `startStaticRedis` canned replies plus a write-then-close helper for truncations, plus dest Pester `/redis` `/dragonfly`.

Is this the best way to solve the issue? Yes versus `master`: pin `*-1` on purpose, do not map it to `redis:miss`, and do not drop live engines.

### Evidence
What I checked:
- `readReply` / `readLine` / `Get` / `parseIntegerReply` / `exec` retry-borrow (`simpleredis/simpleredis.go`)
- Existing malformed tests: `TestEvalNestedArrayIsIssue`, `TestIncrGarbageIntegerPayload` (`simpleredis/simpleredis_test.go`)
- Compose + Pester `/redis` `/dragonfly` already on dest
- Official RESP2 null array (`*-1`) vs null bulk (`knowledge/research/ext_redis_resp_null-array/`)
- Dragonfly KEYS + Lua 5.4 `table.maxn` (`knowledge/research/ext_dragonfly_eval/`)
- PR 24 comments empty; CI run 34649731758 in progress

### Rank-up moves
None.
