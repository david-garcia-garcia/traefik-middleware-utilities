# Test coverage

1. [hard] Critical path untested — `simpleredis/pool.go:116` — `handedOff` defer must return the turn when `takeIdleConn` or `dial` panics after the take; `TestPanicInDoReturnsTurnAndClosesSocket` panics after a successful borrow; `(none)` recovers a panic before handoff
   → Assert a recovered panic in borrow after the turn is taken leaves `inUseTurns` full and `OverFrees` 0
   Status: skipped
   Argument: No dest injection for a panic inside takeIdleConn or dial without a product test hook. Ticket named panic-in-do after a successful borrow as the Traefik path; that test exists. handshake/dial-error tests already cover the non-panic !handedOff frees. Do not add a test-only hook.
