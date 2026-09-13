# Test coverage

1. [hard] Edge case untested — `simpleredis/commands_exec.go:70-71` — `doWithHeldSocket` defers `heldSockets.Add(-1)` so a panic in `do` does not leave the client looking busy forever (design: Traefik unwind restores the count; `release` is still not deferred). `bugPanicAfterBorrow` panics after `borrow` returns and never enters `doWithHeldSocket`. `TestBugLostInUseTurnBricksPoolPermanently` and `TestLostTurnsReportsLeak` stay green if the defer is reverted.
   → Panic inside `do` (or a test double that panics from `doWithHeldSocket`), then assert a later Get succeeds and `LostTurns() > 0`
   Status: done
   Argument: TestDoWithHeldSocketPanicRestoresHeldCount panics on nil netConn inside do.
2. [hard] Critical path untested — `simpleredis/pool.go:136-138` — `heldSockets` around `dial` so a pool-wait waiter during `DialContext` does not refill and dial past `PoolSize`. `TestRecoveryDoesNotFireWhileSocketsBusy` waits until `fake.connections() >= 2` (holders already in `do`). `(none)` times out while dial is in flight.
   → Assert a waiter during a hung dial returns pool-wait `redis:unreachable`, `LostTurns()==0`, and no extra TCP accept
   Status: done
   Argument: TestRecoveryDoesNotFireDuringHungDial against 192.0.2.1.
3. [judgement] Happy path only — `simpleredis/pool.go:172-173` — `recoverLostTurns` returns false when `filled == 0` (second waiter under `turnRecoverMu` sees a full channel). Tests only cover one recovering Get after serial panics. Design named two waiters timing out together.
   → Assert two concurrent waiters after a leak restore once (`LostTurns` equals the gap, not 2×) and do not dial past `PoolSize`
   Status: skipped
   Argument: judgement; mutex serialize is the cap proof, not a second LostTurns equality test.
