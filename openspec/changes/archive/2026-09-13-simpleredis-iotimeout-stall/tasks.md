## 1. Product

- [x] 1.1 Add `commandBudgetLeft(ctx, fallback)` in `simpleredis/resp.go`: `time.Until(ctx.Deadline())`, or `fallback` when `ctx` carries no deadline
- [x] 1.2 In `do`, stamp `SetDeadline` from `commandBudgetLeft(ctx, sr.IOTimeout())` instead of `clampTimeout(ctx, sr.IOTimeout())`
- [x] 1.3 Drop `ioBound` / `ioTimeout` from `ioOrContext`; report `context.DeadlineExceeded` when `ctx` has a deadline and the error is `os.ErrDeadlineExceeded`, so `libraryTimeout` still separates library from caller
- [x] 1.4 Raise `defaultIOTimeout` from 100 ms to 250 ms
- [x] 1.5 Update `Config.IOTimeout`, the zero-Config default list, the worst-case wait, and the `IOTimeout()` accessor doc to say budget input rather than per-operation cap

## 2. Tests

- [x] 2.1 Add `simpleredis/bug3_command_budget_deadline_test.go` with `bug3`-prefixed fakes: streaming bulk beyond `IOTimeout` returns intact; silent mid-reply times out at the budget (elapsed `>= IOTimeout` and `<= budget + slack`); drip returns within budget, `OverFrees()==0`, later Get obtains a turn
- [x] 2.2 Update `TestZeroConfigMaxRetriesIsOneExtra` to assert `IOTimeout() == 250ms`
- [x] 2.3 `go vet ./simpleredis/`
- [x] 2.4 `go test ./simpleredis/ -count=1`
- [x] 2.5 `go test ./... -count=1 -short`
- [x] 2.6 `go test -tags simpleredis_bugs ./simpleredis/ -run TestBugValueLargerThanIOTimeout -v` from the caller checkout that has the tagged file (expect pass after the fix)

## 3. Spec and usage

- [x] 3.1 Update `knowledge/devdocs/std_go_simpleredis.md`: budget numbers (900 ms), and a gotcha stating `IOTimeout` is a budget input and a second per-`Read` deadline must not be added back
- [x] 3.2 `openspec validate --change simpleredis-iotimeout-stall --strict`
