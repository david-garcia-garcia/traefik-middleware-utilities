## Context

Dest `borrow` returns a reused idle socket or a new dial with the same three values, so `exec` cannot tell a dead unused socket from a down peer. Sibling BUG-6 rewrites `takeIdleConn`. Package constraints: Go 1.21, stdlib already used, Yaegi-safe (no generics, no reflection, no new imports). See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Tell `exec` whether the handed socket came from idle, without a fourth `borrow` return.
- After one unused-socket `errUnreachable`, skip idle for the rest of that command and do not consume `MaxRetries` for that one failure.
- Prove sequential recovery in the default suite against a fake that can drop every accepted socket.

**Non-Goals:**
- Pool generation / epoch.
- Discarding leftover idle sockets from `exec`.
- A liveness probe on borrow.
- Touching other packages, `takeIdleConn` body, handshake no-retry, or timeout no-retry.
- Committing the tagged `bugs_production_test.go` files.

## Decisions

1. **`borrow` stays three values; shared body takes `skipIdle`.** Package `borrow(ctx)` calls `borrowSocket(ctx, false)` (name may shift at apply). `exec` calls the shared body with `skipIdle` after a reused unreachable. Tests that already unpack three values stay. Alternative: fourth return or a `borrow(ctx, skipIdle bool)` signature — rejected; sibling PRs and three test call sites unpack three values; the existing nolint is about that order.

2. **Reuse is a return of the shared body, not a field that outlives release.** `exec` reads it in the same iteration that ran `runOnConn`. Alternative: `fromIdle` on `pooledConn` — also fine, but a return does not add a lifetime on the struct. Alternative: epoch on `SimpleRedis` — rejected; moving parts outnumber the bug.

3. **One free unused-socket send, then `skipIdle` stays true for the rest of this `exec`.** Hard bound: the free send happens at most once (`staleReuseRetryUsed`). Remaining attempts still count against `MaxRetries`. A force-dial failure is evidence about the peer. Alternative: consume `MaxRetries` and only skip idle — rejected; Desired says the unused-socket I/O does not consume the budget, and `MaxRetries: -1` would still fail the first sequential Get.

4. **Leave leftover corpses parked.** Each later sequential command spends one corpse then force-dials. Alternative: wipe idle under the mutex — rejected; park race with a fresh socket, and BUG-6 owns that lock.

5. **Default-suite fake is `stalePooledSocketFake` in `stale_pooled_socket_retry_test.go`.** Prefix is bug-specific so sibling branches' fakes do not collide. Reuse `readCommand`, `bulk`, `statusOKReply`, `pooledIdle`, `assertTurnsFullAndNoOverFrees`. Do not add a `killAll` method on `fakeRedis` (`peerCloseFake` only closes the first accept). Warm with simultaneous in-flight Gets (hold channel), then close every accepted fd from the server.

6. **Do not extend `bindCommandDeadline`.** FIN/RST unused-socket EOF is fast; the existing `(maxRetries+1)` hop budget covers the force-dial. Half-open at `MaxRetries: -1` can still exhaust the budget; that is not this defect.

## Risks / Trade-offs

- [Retry pops another corpse] → Mitigation: `skipIdle` stays true for the rest of that command after one unused-socket EOF.
- [Infinite extra sends] → Mitigation: one boolean per `exec`; only unused-socket `errUnreachable` skips the attempt increment.
- [BUG-6 merge on `takeIdleConn`] → Mitigation: skip calling it; do not edit its body.
- [Lost in-use turn] → Mitigation: shared body keeps the existing `handedOff` defer; `runOnConn` still always `release`s; assert `OverFrees() == 0`.
- [Half-open unused socket at `MaxRetries: -1`] → Accepted: full `IOTimeout` then library deadline; not this ticket.

## Migration Plan

Session source plus unit tests. Rollback is revert. No stored data change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
