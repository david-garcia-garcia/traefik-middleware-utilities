# Close abandoned SimpleRedis sockets after panic between borrow and release

PR #66 fixed the permanence of a lost in-use turn after a panic between `borrow` and `release`. On pool-wait timeout, `recoverLostTurnsLocked` refills `inUseTurns` when `len(idleConns) + heldSockets == 0`, and `LostTurns()` makes it diagnosable. `heldSockets` is incremented in `doWithHeldSocket` (around `do`) with a deferred decrement, so a panic inside `do` restores the count while a panic between `borrow` and `do` leaves it at zero.

It did NOT reclaim the socket. Recovery dials a NEW socket; the abandoned one's fd stays open for the process lifetime. So the client recovers its concurrency but accumulates one dead fd per panic, unbounded. On the scenario that motivated the work — a plugin panicking once an hour — that is roughly 24 dead fds a day, and fd exhaustion is the same outage class as the turn exhaustion, just slower.

This ticket is that leak. The debt note is on DestBranch:
  knowledge/debt/2026-09-13-simpleredis-close-panic-leaked-fd.md
There is a second, related debt note you should read for context:
  knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md

The hard part: `heldSockets` is a COUNT, not a handle. You cannot close a socket you have no pointer to, so this needs the checked-out `*pooledConn` reachable from the client (a registry / set / lease table), not just a counter.

The danger is far worse than the leak. Closing a socket that a live command is still using corrupts that command: it reads from a closed socket, or worse the fd is reused. A false positive here is a production incident; the leak is a slow degradation. Be conservative.

The naive predicate is WRONG. "Checked out, not in `idleConns`, and no active command" is ALSO true during two legitimate windows that #66's own debt note documents:
- between `borrow` returning a conn and `doWithHeldSocket` incrementing;
- between the deferred decrement and `release` publishing the socket to `idleConns`.
Closing on that predicate alone will close live sockets.

Suggested safe direction (improve on it if you can, but justify):
- Register each checked-out conn with a checkout timestamp when `borrow` hands it out; deregister in `release` (both the reuse and the destroy path).
- Reclaim ONLY from the existing pool-wait recovery path — do not add a background sweeper goroutine (this package runs interpreted under Yaegi and deliberately avoids extra goroutines; `resp.go` already works around `interp._select` racing on a context channel).
- Reclaim ONLY conns whose checkout is older than a generous bound derived from the command budget the client already computes: `(MaxRetries+1) * (DialTimeout + IOTimeout)` (see `bindCommandDeadline` in `commands_exec.go`). A legitimately in-flight command cannot outlive its own deadline by much, so anything older than that budget is genuinely abandoned. Prefer erring long: a late reclaim is harmless, an early one is not.
- If a registry with timestamps also lets you close #66's held-socket-lease window (the two gaps above) so `recoverLostTurnsLocked` no longer needs the bare `heldSockets` count, take that too and close that debt row per `skill:opd-workflow:Issues` (mark `[x]`, add `Taken:`, delete the debt file). If it does not fall out naturally, LEAVE IT — do not chase it. Closing the fd is the ticket.

Invariants you MUST preserve (all currently green on DestBranch):
- No deferred `release` in `exec`. PR #29 stands.
- `PoolSize` stays the live-socket cap, with only #66's documented bounded transient overshoot.
- `TestOverFreeAccountingStaysBalanced` ends with `len(inUseTurns) == cap(inUseTurns)` and `OverFrees() == 0`.
- `TestPoolWaitTimesOutWithoutExtraDial`: a saturated pool returns `redis:unreachable` within ONE PoolTimeout and opens no extra connection.
- #66's tests stay green: `TestRecoveryDoesNotFireWhileSocketsBusy`, `TestLostTurnsReportsLeak`, `TestDoWithHeldSocketPanicRestoresHeldCount`, `TestRecoveryDoesNotFireDuringHungDial`, plus the ported lost-turn regression test.
- `TestConnectionIsReused`: 25 sequential Gets on exactly 1 TCP connection. No churn.

Tests you must land (permanent, untagged, default suite):
1. After `PoolSize` recovered panics between `borrow` and `release`, the abandoned sockets are eventually CLOSED — assert server-side open sockets settle at a bounded number, not just that turns recover. The fake tracks this; `fake.connections()` and the open-socket helpers are in `simpleredis/fake_redis_test.go`, and #66 already ported a `bugPanicAfterBorrow`-style helper into `simpleredis/pool_test.go`.
2. A live command is NEVER reclaimed: a command genuinely in flight past a pool-wait recovery on another goroutine must complete correctly, with its socket untouched. This is the test that guards the dangerous failure mode — make it convincing, and run it at `-count=5`.
3. A conn in the borrow-to-do or do-to-release gap is not reclaimed.
4. `LostTurns()` (and any new counter you expose) still reports honestly.

Package constraints (openspec/specs/std_go_simpleredis_tcp-session/spec.md):
- simpleredis source imports ONLY the Go standard library. No third-party, no vendored packages.
- No `unsafe`, no cgo, no type parameters (generics).
- Retry jitter stays stdlib `math/rand` `Int63n`, NOT `math/rand/v2`.
- Runs INTERPRETED under Yaegi. Avoid what breaks it: `errors.As` on a package-local struct, `net.Error` type asserts, and interpreted code selecting on a context channel from a goroutine. Prefer plain fields, maps under a mutex, and atomics. Do not add goroutines.
- Pool/timeout/retry knobs are frozen at New on Config. New READ-ONLY accessors are fine; writable exported fields are not.

Say prominently on the delivery card that this PR stacks on #66 and that #66 must merge first.
