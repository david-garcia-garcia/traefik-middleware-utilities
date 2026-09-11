# Test coverage

Ticket job (requirement.md / proposal Why / spec “Stale idle head is closed while the tail stays hot”): on reusable `release`, peel idle-head sockets older than `idleTimeout` and stop at the first still-valid head, so a cold head cannot sit behind a recycled tail.

1. [hard] Edge case untested — `simpleredis/simpleredis.go:258` — peel `break`s when `now.Sub(head.lastUsed) < idleTimeout`; `proveIdleHeadSweep` (`simpleredis/simpleredis_test.go:680`) Gets after two idle so the tail is borrowed and idle is only the stale head, so the still-valid `break` never runs
   → Add a three-idle fake test: backdate head, Get (borrow tail), assert the still-valid middle remains in idle and the stale head is closed
   Status: done
   Argument: TestStaleIdleHeadPeelStopsAtStillValidHead (`73efe0b`).
