## 1. Socket deadline

- [x] 1.1 Add `commandBudgetLeft(ctx)` in `simpleredis/resp.go`: `time.Until(ctx.Deadline())`, or `CommandTimeout` when `ctx` carries no deadline (direct-`do` path)
- [x] 1.2 In `do`, stamp `SetDeadline` from `sr.commandBudgetLeft(ctx)` instead of clamping against a per-operation knob
- [x] 1.3 Drop `ioBound` / `ioTimeout` from `ioOrContext`; report `context.DeadlineExceeded` when `ctx` has a deadline and the error is `os.ErrDeadlineExceeded`, so `libraryTimeout` still separates library from caller
- [x] 1.4 Do not add a per-kernel-`Read` deadline wrapper; the socket deadline stays a single stamp

## 2. Collapse the knobs

- [x] 2.1 Replace `Config.IOTimeout` with `Config.CommandTimeout`; `applyDefaults` fills it when `<= 0`
- [x] 2.2 `defaultIOTimeout` → `defaultCommandTimeout = 900 * time.Millisecond` (dest worst case `(1+1)*(200+250)`)
- [x] 2.3 Rename the client field to `commandTimeout` and the accessor `IOTimeout()` → `CommandTimeout()`; remove `IOTimeout` with no alias
- [x] 2.4 `bindCommandDeadline` binds `now + CommandTimeout` and drops its `maxRetries` parameter; `MaxRetries` bounds attempts only
- [x] 2.5 Leave `Config.DialTimeout` and `dial`'s `net.Dialer{Timeout: ...}` exactly as they are
- [x] 2.6 Update the zero-Config default list, the worst-case-wait sentence, and every doc comment that quoted the product

## 3. Call sites

- [x] 3.1 `e2e/simpleredisprobe/plugin.go`: both `New` calls take `CommandTimeout: 3 * time.Second` (Pester Evals a 500 ms TIME wait; dest budget was 2.4 s)
- [x] 3.2 `windowcounter/repro_lock_during_get_test.go`: `CommandTimeout: 5 * time.Second`
- [x] 3.3 Every `simpleredis` test that set `IOTimeout` sets `CommandTimeout` with a value that preserves that test's intended budget

## 4. Tests

- [x] 4.1 `simpleredis/bug3_command_budget_deadline_test.go`: streaming bulk returns intact inside `CommandTimeout`; silent mid-reply times out at the budget (elapsed `>= CommandTimeout/2` and `<= CommandTimeout + slack`); drip returns within budget, `OverFrees()==0`, later Get obtains a turn
- [x] 4.2 `TestZeroConfigMaxRetriesIsOneExtra` asserts `CommandTimeout() == 900ms` next to `DialTimeout() == 200ms`
- [x] 4.3 `TestBlackHoleGetReturnsWithinOverallDeadline` and `TestHandshakeStallIsBoundedByOverallDeadline` bound on `CommandTimeout`; the stall test drops the `DialTimeout + 2×IOTimeout` fresh-step bound, which the single knob makes vacuous
- [x] 4.4 `TestGetCallerDeadlineIsDeadlineExceededNotRedisTimeout` stays green (sooner caller deadline is `context.DeadlineExceeded`, library budget is `redis:timeout`)
- [x] 4.5 `go build ./...`, `go vet ./...`, `go test ./simpleredis/ -count=1`, `go test ./... -count=1 -short`
- [x] 4.6 Measure at zero Config: 8 MiB streamed in 128 KiB chunks returns intact in ~535 ms (3/3); a peer that goes quiet mid-reply returns `redis:timeout` at 900.2-900.5 ms (3/3)

## 5. Spec and usage

- [x] 5.1 `openspec/specs/std_go_simpleredis_tcp-session/spec.md`: zero-Config defaults, the overall-deadline requirement, the socket-deadline requirement (retitled, with the single-stamp MUST NOT), and both go-redis deviation mentions with the verified pin
- [x] 5.2 `openspec/specs/std_go_simpleredis_resp-commands/spec.md`: the per-hop Eval budget is `CommandTimeout`
- [x] 5.3 `knowledge/devdocs/std_go_simpleredis.md`: worst-case-wait sentence, the numbers, the knob-semantics gotcha, and the probe `CommandTimeout` note; keep the warning against a second per-`Read` deadline
- [x] 5.4 Rename this change folder from `2026-09-13-simpleredis-iotimeout-stall` and rewrite proposal / design / tasks for what shipped, including the two rejected alternatives
