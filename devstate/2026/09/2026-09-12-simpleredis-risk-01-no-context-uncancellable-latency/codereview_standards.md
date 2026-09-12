# Standards

1. [hard] Leave a trail — `simpleredis/commands_exec.go:11` — `exec` now owns the overall deadline and cancel-vs-timeout split (`commandDeadline`, `ctx.Err()` vs `errTimeout`) but the job comment still only says borrow, run, and retry
   → Name overall deadline and cancel classification in the method comment
   Status: done
   Argument: method comment names overall deadline and cancel vs redis:timeout.
2. [hard] Leave a trail — `simpleredis/pool.go:82` — waiter path dropped `Waiter past poolSize: allocate a stoppable timer` and now classifies timer expiry as `errTimeout` vs `errPoolWait` with no block intro (`budgetExpired`)
   → Restore a one-line intro: clamp pool wait to remaining budget; timer expiry is overall timeout when the budget was shorter
   Status: done
   Argument: restored block intro on the waiter path.
