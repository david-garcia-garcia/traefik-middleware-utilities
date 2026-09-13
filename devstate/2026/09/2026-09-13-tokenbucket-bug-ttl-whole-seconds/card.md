Developer review: in progress — 2026-09-13T06:24:30Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `NewMemory` and `NewRedis` reject a `ttl` that is not a whole number of seconds (`errTTL`). 2s still constructs. `tokenbucket/ttl_truncation_test.go` proves the reject.

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
Apply landed locally; CI on the apply head is still running.

Priority: P2 — stores can expire the same key at different times when ttl is not a whole second
Reviewed head: 7e02c79
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI in progress after the apply |
| CI proof | 3/6 | Checks queued on the stub PR |
| Local tests proof | N/A | Remote PR; CI covers proof (`localTests: passed`) |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-tokenbucket-bug-ttl-whole-seconds pushed | `git push` `7e02c79` |
| OpenSpec | tokenbucket-ttl-whole-seconds | `openspec/changes/tokenbucket-ttl-whole-seconds/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/57 | pr-host Create |
| CI | build 34742666190 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742666190 | pr-host CI (head `7e02c79`) |
| Local tests | passed | `go test -short -count=1 ./...` |
| PR comments | no comments | inventory empty |

## Specs
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-ttl-whole-seconds/openspec/changes/tokenbucket-ttl-whole-seconds/proposal.md) — modified
- [std_go_tokenbucket_lua-eval](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-ttl-whole-seconds/openspec/changes/tokenbucket-ttl-whole-seconds/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-tokenbucket-bug-ttl-whole-seconds`, stub PR #57. Apply rejected fractional ttl at `validateClock`. Code review is next.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| What exact `errTTL` string after the whole-second gate? | additive asked | assumed — keep sentinel `errTTL`; set the text to `tokenbucket: ttl must be a whole number of seconds (at least 1s)`. One check: `ttl < time.Second || ttl%time.Second != 0`. | explore |
| Where does the adapted repro live after the reshape? | additive asked | assumed — land `tokenbucket/ttl_truncation_test.go` (drop `repro_` once it is the contract). Adapt `Allow` to dest `(ctx, key) (bool, time.Duration, error)`. After the fix, assert `errors.Is(err, errTTL)` for 1500ms on both constructors; keep 2s accepted. Do not keep ARGV=`1` / Memory-at-+1200ms as the post-fix contract. | explore |
| Is NewRedis(1500ms) a dedicated case or only Memory plus shared `validateClock`? | additive asked | assumed — both constructors call `validateClock`; the test calls both NewMemory and NewRedis with 1500ms and `errors.Is(..., errTTL)`. 2s still succeeds on both. Dest ARGV/`+1200ms` proof exists only in the failing-first version of that test. | explore |

## Before merge
- [x] Land tokenbucket tests that fail on dest for New(1500ms) accepted, Redis ARGV ttl 1, Memory still live at +1200ms, then reject fractional ttl at validateClock and keep 2s accepted
- [ ] Archive whole-second ttl into the live tokenbucket allow and lua-eval specs

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 7e02c792d64f6adba2c6952aa693f0257fe055f8 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Whole-second `validateClock` (same `errTTL`) so Memory `now.Add(ttl)` and Redis EXPIRE seconds are the same lifetime.

Do we have a high-confidence way to reproduce? Yes — dest FAIL: New accepted 1500ms, ARGV ttl `"1"`, Memory at +1200ms admitted 1 want 2. After the gate, `go test -short ./tokenbucket/` PASS.

Is this the best way to solve the issue? Yes — PEXPIRE or flooring Memory while New succeeds would leave Redis unable to represent the Duration New accepted.

### Evidence
What I checked:
- Dest FAIL `TestTTLTruncation_MemoryVsRedis` (`fbfed5c`): ARGV ttl=`"1"`; Memory at +1200ms admitted 1 want 2
- Apply `3afcc2e`: `validateClock` `ttl < time.Second || ttl%time.Second != 0`; reshaped test `TestTTLTruncation_RejectsFractionalTTL`
- `go test -short -count=1 ./...` passed
- `openspec validate tokenbucket-ttl-whole-seconds --type change --strict` valid
- CI run 34742666190 queued

### Rank-up moves
None.
