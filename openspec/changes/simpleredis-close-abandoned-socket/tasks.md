## 1. Registry and reclaim

- [ ] 1.1 Add `checkedOut` map and `checkedOutMu` on `SimpleRedis`; export `AbandonedClosed() int64` next to `LostTurns()`
- [ ] 1.2 Register the conn with `time.Now()` at the end of `takeIdleOrDial` (idle reuse and successful dial); deregister at the start of `release` on both branches
- [ ] 1.3 Share the command-budget duration with `bindCommandDeadline` (after `retryLimits`, `(maxRetries+1)*(DialTimeout+IOTimeout)`)
- [ ] 1.4 On `borrowAfterPoolWait`, close and count registry entries whose checkout age is at least that budget, before the turn-refill gates
- [ ] 1.5 Keep `recoverLostTurnsLocked` refill as idle empty and `heldSockets == 0`; do not `defer sr.release`; do not add a goroutine

## 2. Tests

- [ ] 2.1 After `PoolSize` `bugPanicAfterBorrow` and one command budget, Get succeeds and `waitOpenSocketsEqual` settles at a bounded live count (fail-before then pass-after)
- [ ] 2.2 Live in-flight command (`holdGetsForTest`) is not closed when another goroutine hits pool-wait recovery; run `-count=5`
- [ ] 2.3 Same-package gap tests: borrow-and-hold, and do-then-hold-before-release, are not reclaimed while checkout is younger than the budget
- [ ] 2.4 `LostTurns()` still reports the leak; `AbandonedClosed()` is 0 before the budget and at least `PoolSize` after close
- [ ] 2.5 Keep green: `TestBugLostInUseTurnBricksPoolPermanently`, `TestRecoveryDoesNotFireWhileSocketsBusy`, `TestLostTurnsReportsLeak`, `TestDoWithHeldSocketPanicRestoresHeldCount`, `TestRecoveryDoesNotFireDuringHungDial`, `TestOverFreeAccountingStaysBalanced`, `TestPoolWaitTimesOutWithoutExtraDial`, `TestConnectionIsReused`
- [ ] 2.6 `go vet ./simpleredis/` and `go test -count=1 -timeout 300s ./simpleredis/`
- [ ] 2.7 `go test -count=5 -timeout 600s -run "Pool|Recover|Turn|Abandon|Reclaim" ./simpleredis/` (no local `-race`; no C toolchain)

## 3. Usage packet, debt, spec

- [ ] 3.1 Gotcha on `knowledge/devdocs/std_go_simpleredis.md`: abandoned fds close on pool-wait when checkout age is at least the command budget; `AbandonedClosed()` diagnoses it
- [ ] 3.2 Delete `knowledge/debt/2026-09-13-simpleredis-close-panic-leaked-fd.md`; mark that dest row taken if present. Leave `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md`
- [ ] 3.3 Confirm the change delta `std_go_simpleredis_tcp-session` matches the landed tests
- [ ] 3.4 Run `openspec validate --change simpleredis-close-abandoned-socket --strict`
