Developer review: ready for review — 2026-09-11T22:02:33Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Dirty RESP decode returns `redis:issue?` or I/O and stays out of the idle pool; `*-1` stays `redis:issue?` not `redis:miss`; Get/Incr arity mismatch keeps a clean conn pooled; live Get-miss (`$-1`) is asserted on `/redis` and `/dragonfly`.

**End users.** None.

## Motivation
On `master`, SimpleRedis already treats empty lines, unknown type bytes, bad array counts (including `*-1`), missing CR, and truncated elements as `redis:issue?` and marks the socket dirty. Almost none of those branches ran. Only nested-array and garbage-integer cases existed, and they did not assert the idle list is empty. A proxy, TLS-as-plaintext, or HTTP on the Redis port is how those paths actually fire.

If we do not merge the coverage, a later decoder change can ship with those failure modes unproven, and a reused poisoned connection can contaminate the next request. Live `/redis` and `/dragonfly` already proved the happy path; they did not prove Get-miss (`$-1`) beside fake `*-1` as issue.

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
Implement is on the stub PR and CI succeeded. 0 items remain.

Priority: P3 — tests and decoder proof, no current operator or user harm
Reviewed head: 6b03af5
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | CI succeeded; no open PR comments |
| CI proof | 6/6 | Lint, Test, and Integration Tests succeeded |
| Local tests proof | N/A | Remote PR; CI is the proof axis |
| Review resolution | 6/6 | No PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-11-simpleredis-test-06-malformed-reply pushed | `git` / origin |
| OpenSpec | simpleredis-malformed-reply | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/24 | pr-host List/Create |
| CI | build 34652009660 success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34652009660 | pr-host CI |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
- [std_go_simpleredis_resp-commands](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-11-simpleredis-test-06-malformed-reply/openspec/changes/simpleredis-malformed-reply/proposal.md) — modified

## Deviations from the ask
- taken: uniform `idle==0` on `Get` / `parseIntegerReply` arity mismatch → `idle==0` only on dirty decoder rows — `simpleredis/simpleredis.go` (`Get`, `parseIntegerReply`, `release`) — those checks run after a clean `readReply`. Requester: not asked.

## Follow-up issues
None.

## How this fits together
Local test-06 finding → implement `simpleredis-malformed-reply` on `2026-09-11-simpleredis-test-06-malformed-reply` → PR 24 → CI run 34652009660 succeeded.

## Explore Decisions
None.

## Before merge
None.

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 6b03af5729f6a8d22de94a8d64db01d8296a02e0 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: pin `*-1` as issue (no verb receives a null array), prove poison with a fake-server table plus write-then-close truncations, and keep live Redis and Dragonfly Get-miss so a decoder change cannot ship unproven.

Do we have a high-confidence way to reproduce? Yes, table-driven `startStaticRedis` plus write-then-close truncations, a one-accept retry-borrow helper, and CI Pester `/redis` `/dragonfly`.

Is this the best way to solve the issue? Yes versus `master`: keep dest `count < 0` as issue, do not map `*-1` to `redis:miss`, and do not drop live engines.

### Evidence
What I checked:
- `go test -timeout 2m -count=1 ./...` passed locally (SHA 6b03af5)
- `openspec validate simpleredis-malformed-reply --strict --type change` — valid
- Lint success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34652009660/job/103436090085
- Test success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34652009660/job/103436090297
- Integration Tests success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34652009660/job/103436090305
- PR 24 comments: none

### Rank-up moves
None.
