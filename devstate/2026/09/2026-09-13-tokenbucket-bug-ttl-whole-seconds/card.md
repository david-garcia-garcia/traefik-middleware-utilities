Developer review: in progress — 2026-09-13T06:20:48Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `tokenbucket-ttl-whole-seconds` folds whole-second `ttl` into `std_go_tokenbucket_allow` and `std_go_tokenbucket_lua-eval`. Product constructors are still dest until implement.

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
Propose is apply-ready; product apply has not started.

Priority: P2 — stores can expire the same key at different times when ttl is not a whole second
Reviewed head: fa2605f
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | CI in progress; no product apply yet |
| CI proof | 3/6 | Checks queued on the stub PR |
| Local tests proof | N/A | Before implement (`localTests: none`) |
| Review resolution | 6/6 | OPEN PR, no review comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-tokenbucket-bug-ttl-whole-seconds pushed | `git push` `fa2605f` |
| OpenSpec | tokenbucket-ttl-whole-seconds | `openspec/changes/tokenbucket-ttl-whole-seconds/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/57 | pr-host Create |
| CI | build 34742515584 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34742515584 | pr-host CI (head `fa2605f`) |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-ttl-whole-seconds/openspec/changes/tokenbucket-ttl-whole-seconds/proposal.md) — modified
- [std_go_tokenbucket_lua-eval](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-tokenbucket-bug-ttl-whole-seconds/openspec/changes/tokenbucket-ttl-whole-seconds/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket, branch `2026-09-13-tokenbucket-bug-ttl-whole-seconds`, stub PR #57. Propose folded two existing spec leaves. Implement is next (tests first).

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| What exact `errTTL` string after the whole-second gate? | additive asked | assumed — keep sentinel `errTTL`; set the text to `tokenbucket: ttl must be a whole number of seconds (at least 1s)`. One check: `ttl < time.Second || ttl%time.Second != 0`. | explore |
| Where does the adapted repro live after the reshape? | additive asked | assumed — land `tokenbucket/ttl_truncation_test.go` (drop `repro_` once it is the contract). Adapt `Allow` to dest `(ctx, key) (bool, time.Duration, error)`. After the fix, assert `errors.Is(err, errTTL)` for 1500ms on both constructors; keep 2s accepted. Do not keep ARGV=`1` / Memory-at-+1200ms as the post-fix contract. | explore |
| Is NewRedis(1500ms) a dedicated case or only Memory plus shared `validateClock`? | additive asked | assumed — both constructors call `validateClock`; the test calls both NewMemory and NewRedis with 1500ms and `errors.Is(..., errTTL)`. 2s still succeeds on both. Dest ARGV/`+1200ms` proof exists only in the failing-first version of that test. | explore |

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
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | fa2605fde1cbff53f6e70932cb446865c72ddf16 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Dest still accepts 1500ms then disagrees on lifetime; the ticket’s whole-second `validateClock` (same `errTTL`) is the how.

Do we have a high-confidence way to reproduce? Yes — dest NewMemory/NewRedis with 1500ms succeed; Redis EVAL ARGV ttl is `"1"`; Memory still admits at +1200ms.

Is this the best way to solve the issue? Yes — PEXPIRE or flooring Memory while New succeeds would leave Redis unable to represent the Duration New accepted.

### Evidence
What I checked:
- FindSpecHost fold `std_go_tokenbucket_allow` and `std_go_tokenbucket_lua-eval` (high)
- `openspec validate tokenbucket-ttl-whole-seconds --type change --strict` valid
- validate_artifact_names OK
- Stub PR #57, CI run 34742515584 queued on `fa2605f`

### Rank-up moves
None.
