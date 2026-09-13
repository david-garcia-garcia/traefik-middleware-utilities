## Context

Dest `borrow` returns a reused idle socket or a new dial with the same three values, so `exec` cannot tell a dead unused socket from a down peer. Sibling BUG-6 rewrites `takeIdleConn`. Package constraints: Go 1.21, stdlib already used, Yaegi-safe (no generics, no reflection, no new imports). See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Tell `exec` whether the handed socket came from idle, without a fourth `borrow` return.
- After one unused-socket `errUnreachable`, skip idle for the rest of that command so the retry dials instead of popping another corpse.
- Prove sequential recovery in the default suite against a fake that can drop every accepted socket.

**Non-Goals:**
- Pool generation / epoch.
- Discarding leftover idle sockets from `exec`.
- A liveness probe on borrow.
- Touching other packages, `takeIdleConn` body, handshake no-retry, or timeout no-retry.
- Committing the tagged `bugs_production_test.go` files.

## Decisions

1. **`borrowSocket` is the one borrow path.** `skipIdle` skips the unused list. Tests that used three-value `borrow` call `borrowSocket(ctx, false)` and drop `fromIdle`. Alternative: keep a three-value `borrow` wrapper — rejected; after exec moved, it had no production callers.

2. **Reuse is a return of the shared body, not a field that outlives release.** `exec` reads it in the same iteration that ran `runOnConn`. Alternative: `fromIdle` on `pooledConn` — also fine, but a return does not add a lifetime on the struct. Alternative: epoch on `SimpleRedis` — rejected; moving parts outnumber the bug.

3. **Remaining attempts skip idle; MaxRetries still counts.** After `fromIdle && unreachable`, `skipIdle` stays true for the rest of this `exec` loop. No free extra send: on this platform a dead unused socket can fail after `write` succeeds, which is indistinguishable from a lost reply. Alternative: one extra send when `wroteCommand` is false — rejected; dest dead-idle Gets still write successfully then read EOF. Alternative: pre-write peek of the kernel receive buffer — rejected; that is BUG-2's surface and more moving parts than skipIdle.

4. **Leave leftover corpses parked.** Each later sequential command spends one corpse then force-dials. Alternative: wipe idle under the mutex — rejected; park race with a fresh socket, and BUG-6 owns that lock.

5. **Default-suite fake is `peerDropAllFake` in `peer_drop_all_test.go`.** Prefix is bug-specific so sibling branches' fakes do not collide. Reuse `readCommand`, `bulk`, `statusOKReply`, `pooledIdle`, `assertTurnsFullAndNoOverFrees`. Do not add a `killAll` method on `fakeRedis` (`peerCloseFake` only closes the first accept). Warm with simultaneous in-flight Gets (hold channel), then close every accepted fd from the server.

6. **Do not extend `bindCommandDeadline`.** Recovery uses the existing retry slot. `MaxRetries: -1` stays one send.

## Risks / Trade-offs

- [Retry pops another corpse] → Mitigation: `skipIdle` stays true for the rest of that command after one unused-socket EOF.
- [BUG-6 merge on `takeIdleConn`] → Mitigation: skip calling it; do not edit its body.
- [Lost in-use turn] → Mitigation: shared body keeps the existing `handedOff` defer; `runOnConn` still always `release`s; assert `OverFrees() == 0`.
- [MaxRetries -1 after a full vintage drop] → Accepted: one send, that command fails; dest Lost-reply Incr with MaxRetries off stays. Default MaxRetries recovers sequential Gets.

## Migration Plan

Session source plus unit tests. Rollback is revert. No stored data change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
