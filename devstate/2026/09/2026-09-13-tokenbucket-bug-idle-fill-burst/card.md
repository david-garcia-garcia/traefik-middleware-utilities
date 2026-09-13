Developer review: in progress — 2026-09-13T06:12:12Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Motivation
A new token-bucket key is supposed to start full (`burst`) then consume 1. On master, both Memory and Redis Lua seed missing state as `tokens=0` `last=0` and refill from elapsed since Unix epoch.

That fill is empty when the clock is `Unix(0,0)` (elapsed 0, first consume goes to `-1`). It is also short of burst when `burst` is huge (e.g. `1e12`) and `elapsed*rate` from epoch is still below burst — first Allow leaves about `1.7e9` tokens instead of `burst-1`. Existing unit tests freeze at `Unix(1_700_000_000)` with small burst, so they pass without proving a true full start.

If this stays unmerged, a new or TTL-expired key can under-grant relative to the Allow spec whenever elapsed-from-epoch cannot cover burst. Typical small burst at wall-clock now coincidentally looks full.

```mermaid
sequenceDiagram
  participant Caller
  participant Store
  Note over Store: master missing key
  Caller->>Store: Allow new key
  Store->>Store: tokens 0 last 0
  Store->>Store: elapsed = now minus epoch
  alt clock is Unix epoch
    Store->>Caller: tokens -1 after consume
  else burst 1e12 and elapsed times rate below burst
    Store->>Caller: tokens elapsed times rate minus 1 not burst minus 1
  end
```

## Merge readiness
Prepare grounded the ticket and opened the stub PR. Product apply has not started. 5 items remain.

Priority: P2 — real under-grant on new keys when elapsed-from-epoch cannot cover burst; typical small burst at wall-clock now coincidentally fills
Reviewed head: 6d2565a
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Stub PR exists; no product apply; CI not seen |
| CI proof | 1/6 | Pushed; checks not seen |
| Local tests proof | N/A | Before implement |
| Review resolution | 6/6 | OPEN PR; no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-tokenbucket-bug-idle-fill-burst pushed | `git` origin/2026-09-13-tokenbucket-bug-idle-fill-burst |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/56 | pr-host Create |
| CI | not seen | pr-host CI |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket 2026-09-13-tokenbucket-bug-idle-fill-burst is on its branch from origin/master. Stub PR 56 is the durable card host. CI has not been seen yet.

## Explore Decisions
None.

## Before merge
- [ ] Tests that fail on dest (`epoch_clock` burst 5; `huge_burst_elapsed_below_burst` burst 1e12), then Lua empty-hash and Memory new-entry fill, then PASS
- [ ] Memory and Redis/fake still agree under `-short`
- [ ] CI succeeded on PR 56
- [x] Stub PR open
- [x] Requirement grounded on dest Lua and Memory

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
| Reviewed head | 6d2565a021d7c73f6efbb8486bc9ec06b7d287bd | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: dest still uses Traefik `last=0` elapsed-from-epoch; the agreed fill is missing state = full burst then consume 1 on both stores.

Do we have a high-confidence way to reproduce? Yes, dest `consumeOne` at `last=0` with `nowMicro=0` or burst `1e12` at `Unix(1_700_000_000)`; caller repro file must be adapted to dest `Allow(ctx, key)`.

Is this the best way to solve the issue? Yes — seed Lua empty hash and Memory new `memEntry` full, keep `consumeOne` as consume math, match fake missing hash to Lua.

### Evidence
What I checked:
- dest `tokenbucket/lua.go` empty HGETALL leaves tokens=0 last=0 (`git ls-tree origin/master tokenbucket/`)
- dest `tokenbucket/memory.go` `entry = &memEntry{}` then `consumeOne` (`d69f89d`)
- dest `tokenbucket/fake_redis_test.go` missing hash seeds zeros then `consumeOne`
- dest `openspec/specs/std_go_tokenbucket_allow/spec.md` Idle fills to burst
- GitHub identity `get_me` login david-garcia-garcia email deivid.garcia.garcia@gmail.com
- OPEN PR 56; issue comments []; review threads []

### Rank-up moves
None.
