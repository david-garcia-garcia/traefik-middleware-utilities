# Requirement
IssueKey: 2026-09-12-simpleredis-api-01-sentinels-not-exported

## Problem
`simpleredis` exports error *strings* (`RedisMiss`, `RedisUnreachable`, …) but keeps every error *value* unexported. Outside the package, `errors.Is` is impossible; the only match is `err.Error() == token`. `windowcounter` does that for a Redis miss, which is the normal “counter absent, treat as zero” path. Pool wait and a dead server share the same `Error()` text, so callers also cannot tell them apart.

## Current (code)
- `simpleredis/simpleredis.go:12-28` — exported consts `RedisUnreachable`, `RedisMiss`, `RedisTimeout`, `RedisNoAuth`, `RedisIssue`. Unexported vars `errUnreachable`, `errPoolWait` (same `Error()` text `redis:unreachable`, distinct values), `errMiss`, `errTimeout`, `errNoAuth`, `errIssue`.
- `windowcounter/limiter.go:267-274` — `getCount` treats miss as zero via `err.Error() == simpleredis.RedisMiss`; any other error is a hard fail.
- `simpleredis/commands_exec.go:83-105` — `shouldRetry` / `isUnreachable` identity-compare `err == errUnreachable`. `errPoolWait` is therefore not retried (same text, different value).
- `simpleredis/commands_exec_test.go:283-289` — `TestShouldRetryPoolWaitIsFalse` pins `shouldRetry(errPoolWait) == false` and `shouldRetry(errUnreachable) == true`.
- `simpleredis/resp.go:200-205` — `ioError` already uses `errors.Is` for `os.ErrDeadlineExceeded` (Yaegi-safe), then returns the unexported sentinels.
- `simpleredis/yaegi_test.go` `clientprobeSrc` — interpreted client; no `errors.Is` on package sentinels and no `IsMiss` / `IsUnreachable` / `IsPoolWait`.
- Exported `ErrMiss` / `ErrUnreachable` / `ErrPoolWait` and `IsMiss` / `IsUnreachable` / `IsPoolWait` — not found.

## Desired
1. Export sentinel values (`ErrUnreachable`, `ErrMiss`, `ErrTimeout`, `ErrNoAuth`, `ErrIssue`, `ErrPoolWait`) and keep the string consts for display and legacy text matching.
2. `ErrPoolWait` wraps `ErrUnreachable` so `errors.Is(ErrPoolWait, ErrUnreachable)` is true while `IsPoolWait` still distinguishes them.
3. Internal names stay aliases of the exported values so in-package call sites need not change.
4. `shouldRetry(ErrPoolWait)` stays false and `shouldRetry(ErrUnreachable)` stays true (keep identity, or check `ErrPoolWait` before `ErrUnreachable`).
5. Add `IsMiss`, `IsUnreachable`, `IsPoolWait` (`errors.Is`).
6. Convert `windowcounter/limiter.go:271` to `simpleredis.IsMiss(err)`.
7. Tests: wrap each sentinel and assert `errors.Is` + predicate still match while `err.Error() == token` does not; pin the `ErrPoolWait` / `ErrUnreachable` / `shouldRetry` relationship; a `windowcounter` test where `Get` returns a wrapped miss and the limiter still treats the counter as zero; add `errors.Is` and one predicate to `clientprobe` in `simpleredis/yaegi_test.go`.

## Affected
- `simpleredis/simpleredis.go` (exported vars + predicates).
- `simpleredis/commands_exec.go` (`shouldRetry` / `isUnreachable` identity vs wrap).
- `simpleredis` tests (`commands_exec_test.go` and a wrapping test).
- `simpleredis/yaegi_test.go` (`clientprobe`).
- `windowcounter/limiter.go` (`getCount` miss path).
- `windowcounter` tests (wrapped-miss Get).

## Out of scope
- Other `simpleredisfixes2` findings (`bug-05` wrapping `parseEvalInt`, `risk-04` reply shapes, and the rest of that folder).
- Converting other `err.Error() ==` / `!=` matches (`e2e/simpleredisprobe/plugin.go:210`, package-internal tests, `tokenbucket` test assertions).
- Changing `windowcounter/limiter.go:278` / `parseEvalInt` from `errors.New(simpleredis.RedisIssue)` to wrapping a sentinel.
- Rewriting `shouldRetry` onto `errors.Is` except as needed to keep pool-wait unretried.

## Unknowns
- Whether interpreted `clientprobe` can call `errors.Is` on the exported vars, or only the predicates — ticket expects both because `ioError` already uses `errors.Is` under Yaegi.
- How the `windowcounter` wrapped-miss test injects a wrapped `Get` error (test double vs existing fake).

## Tensions
- Ticket rates this judgement: string match works today; it becomes a correctness bug the first time any path wraps (`%w`). Dest has no wrapping on these sentinels yet.
- Ticket’s `ErrPoolWait = fmt.Errorf("%w", ErrUnreachable)` makes `errors.Is(pool, unreachable)` true. Dest `isUnreachable` is identity (`err == errUnreachable`). If a later edit switches that to `errors.Is`, pool waits would start retrying. The ticket asks to keep identity or check pool wait first, and to pin that with a test.
- Ticket cites `bug-05` / `risk-04` as why wrapping should become possible. Those asks are not this ticket; this change only unblocks them.
