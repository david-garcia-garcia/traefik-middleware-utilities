Developer review: in progress — 2026-09-13T06:12:15Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Motivation
On dest, `NewMemory` and `NewRedis` share one `validateClock`. That gate only rejects `ttl` under 1s, so 1500ms is a legal constructor argument. Memory then expires the key at `now.Add(ttl)` (1.5s). Redis EVAL ARGV is `int64(ttl / time.Second)`, so Traefik’s `EXPIRE` is 1.

The same Allow sequence is supposed to mean the same thing on both stores. With 1500ms it does not: Memory still holds the depleted bucket at +1200ms while Redis has already been told to drop the key after 1s. New accepted a Duration Redis cannot represent.

If this stays unmerged, a caller that passes a fractional-second ttl gets different lifetimes depending on which store is wired. The agreed how is reject that ttl at `validateClock` (same `errTTL`, still `>= 1s`, whole seconds only). Do not PEXPIRE, rewrite Lua, or floor Memory while New(1500ms) succeeds.

```mermaid
sequenceDiagram
  participant New
  participant Memory
  participant Redis
  New->>New: ttl=1500ms accepted
  New->>Memory: expireAt = now + 1.5s
  New->>Redis: EXPIRE ARGV 1
  Note over Memory: still live at +1200ms
  Note over Redis: key gone around 1s
```

## Merge readiness
Prepare is grounded; product apply has not started. Explore is next.

Priority: P2 — stores can expire the same key at different times when ttl is not a whole second
Reviewed head: 271ce43
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI still running; no product apply yet |
| CI proof | 3/6 | Checks in progress on the stub PR |
| Local tests proof | N/A | Before implement (`localTests: none`) |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-tokenbucket-bug-ttl-whole-seconds pushed | `git push` `271ce43` |
| OpenSpec | none | no change folder |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/57 | pr-host Create |
| CI | build 34742111059 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742111059 | pr-host CI (head `271ce43`) |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-tokenbucket-bug-ttl-whole-seconds` from `origin/master` (tokenbucket present). Stub PR #57 is the durable card. Next is explore.

## Explore Decisions
None.

## Before merge
- [ ] Land tokenbucket tests that fail on dest for New(1500ms) accepted, Redis ARGV ttl 1, Memory still live at +1200ms, then reject fractional ttl at validateClock and keep 2s accepted
- [ ] Fold whole-second ttl into the tokenbucket allow spec and usage packet

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
| Reviewed head | 271ce43aedc4553106267468243ae8d195a813a6 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Dest still accepts 1500ms then disagrees on lifetime; the ticket’s whole-second `validateClock` (same `errTTL`) is the how.

Do we have a high-confidence way to reproduce? Yes, NewMemory/NewRedis with 1500ms, Redis EVAL ARGV ttl `"1"`, Memory not expired at +1200ms.

Is this the best way to solve the issue? Yes — PEXPIRE or flooring Memory while New succeeds would leave Redis unable to represent the Duration New accepted.

### Evidence
What I checked:
- `tokenbucket/` exists on `origin/master` at `d69f89d` (`git ls-tree`)
- `validateClock` / `ttlSeconds` (`tokenbucket/clock.go`); `expireAt = now.Add(ttl)` (`tokenbucket/memory.go`); Redis ARGV (`tokenbucket/redis.go`); Lua `expire` (`tokenbucket/lua.go`)
- Dest tests reject `ttl=1ms` only (`tokenbucket/limiter_test.go`); no 1500ms case
- Stub PR #57, comment inventory empty, CI run 34742111059 in progress

### Rank-up moves
None.
