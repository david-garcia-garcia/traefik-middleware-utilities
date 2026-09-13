## Context

DestBranch (PR #66) refills `inUseTurns` on pool-wait when idle is empty and `heldSockets` is 0. `heldSockets` is a count. After a panic between `borrow` and `release` the `*pooledConn` is unreachable, so recovery dials and the abandoned fd stays open. Proceed policies: `devstate/explore.md`. See proposal.md for why. Yaegi: stdlib only, no extra goroutine, no generics.

## Goals / Non-Goals

**Goals:**
- Keep checked-out sockets reachable from the client with a checkout timestamp.
- Close those sockets from the existing pool-wait path when age is at least the command budget.
- Prove a live command and a gap conn are never closed.
- Keep #66 turn refill and `LostTurns()` as they are.

**Non-Goals:**
- `defer sr.release` in `exec`.
- A sweeper goroutine.
- Replacing `heldSockets` for turn refill (held-socket-lease debt stays).
- Closing checked-out sockets from `Close()` (in-flight commands still finish).
- Cherry-pick or merge #67 / #68 / #69; edit `simpleredis/BUGS.md`.

## Decisions

1. **Registry map, not a count.** `map[*pooledConn]time.Time` under `checkedOutMu`. Register at the end of `takeIdleOrDial` (idle reuse and successful dial) so #67's later `borrow` 3-tuple does not have to be pre-empted. Deregister at the start of `release` on both branches. Alternative: a slice of leases — slower deregister, same Yaegi constraints.

2. **Age bound = `bindCommandDeadline` budget.** Shared helper: after `retryLimits`, `(maxRetries+1)*(DialTimeout()+IOTimeout())`. Close when `now.Sub(checkedOutAt) >= budget`. Alternative: 2× budget — safer but slower leak; ticket named this formula. Alternative: close whenever `heldSockets==0` — closes a gap-live socket; rejected.

3. **Reclaim on `borrowAfterPoolWait` before the refill gates.** Close old registry entries even when idle is non-empty or `heldSockets != 0`, so a later waiter can still close leftover fds. Turn refill stays `recoverLostTurnsLocked`: idle empty and `heldSockets == 0`. Alternative: block refill on young checkouts — would fail `TestBugLostInUseTurnBricksPoolPermanently` because panic-abandoned entries are young.

4. **`AbandonedClosed()` atomic.** Increment once per closed fd. Read-only next to `LostTurns()`.

5. **Tests in `simpleredis/pool_test.go` (untagged).** Reuse `bugPanicAfterBorrow`, `holdGetsForTest` / `waitOpenSocketsEqual`. Same-package gap test: `borrow` and hold; sibling `doWithHeldSocket` then hold before `release`. Live-command test at `-count=5`.

## Risks / Trade-offs

- [Close a live command] → Mitigation: age bound from the library deadline; live-command and gap tests; prefer late reclaim.
- [Burst panics then immediate Get leaves fds until the next old-enough pool-wait] → Accepted; "eventually" is after one budget. Peak extra fds is bounded by panics since last aged recovery, not process lifetime.
- [Two waiters double-close] → Mitigation: delete from the map under the mutex before `close()`; `pooledConn.close` is safe to call after a failed command.
- [Yaegi map + mutex] → Dest already uses maps and `sync.Mutex` (`idleConnsMu`). No extra goroutine.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy key. This PR stacks on #66; #66 must merge first. If #66 moves, Sync against `DestBranch`.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
