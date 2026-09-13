# Standards

1. [hard] Symmetry and consistency — `simpleredis/pool.go:136` — `heldSockets` around `dial` is `Add(1)` then `Add(-1)` with no defer; `doWithHeldSocket` restores the count if `do` panics (`defer sr.heldSockets.Add(-1)`). `dial` runs that same `do` for AUTH/SELECT, so a Yaegi panic there leaves `heldSockets > 0` and recovery never refills.
   → Defer the decrement around `dial` the same way `doWithHeldSocket` defers around `do`
   Status: done
   Argument: defer heldSockets.Add(-1) around dial in takeIdleOrDial.
2. [hard] Leave a trail — `simpleredis/pool.go:112` — the pool-wait timeout path now calls `recoverLostTurns` then takes a turn, with no block intro next to the other `borrow` comments (`Uncontended borrow…`, `Waiter past poolSize…`, `Idle miss…`)
   → Comment that a timed-out waiter restores leaked turns then takes one, or still returns pool-wait
   Status: done
   Argument: comment on the timer.C path plus borrowAfterPoolWait.
