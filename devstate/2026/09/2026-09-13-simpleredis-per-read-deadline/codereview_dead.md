# Dead

1. [judgement] Test harness branch — `simpleredis/resp.go:112` — `commandBudgetLeft` `if !ok { return sr.CommandTimeout() }` has no production caller path: `rg '\.do\('` shows production only `commands_exec.go` and `pool.go` (AUTH/SELECT), all with ctx already deadline-bound by `bindCommandDeadline`; the sole undeadlined `do` is `clientIDOnConnForTest` → `sr.do(context.Background(), …)` in `simpleredis/simpleredis_e2e_test.go:181`
   → Documented direct-`do` test seam (design decision 6); optional tighten by giving the e2e helper a `WithDeadline` ctx and dropping the `!ok` arm so `commandBudgetLeft` is always `time.Until`
   Status: skipped
   Argument: judgement, and the tighten it proposes is ruled out by the ask, which requires `do` to stay correct when called with a deadline-free ctx from a test. Dropping the `!ok` arm makes `time.Until(zero)` negative, so `do` would return `redis:timeout` on any deadline-free ctx. The branch stays as the documented direct-`do` seam. Confirmed no leftover `IOTimeout` / `defaultIOTimeout` / `ioTimeout` / `clampTimeout` in production code.
