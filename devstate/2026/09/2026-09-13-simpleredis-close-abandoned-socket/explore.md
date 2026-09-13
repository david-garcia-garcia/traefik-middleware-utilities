# Explore
IssueKey: 2026-09-13-simpleredis-close-abandoned-socket

## Concepts

**Checkout registry.** A map from `*pooledConn` to checkout time on the client, guarded by a mutex. `borrow` (via `takeIdleOrDial`) records the conn when it hands the socket out. `release` removes it on both the reuse path and the destroy path. This is the owner of “which sockets are checked out,” not `heldSockets` (a count with no pointer).

**Command budget.** The same duration `bindCommandDeadline` already binds: after `retryLimits`, `(maxRetries+1)*(DialTimeout()+IOTimeout())`. Frozen at New. A live command’s context and socket `SetDeadline` cannot outlive this by more than scheduling slack (`contextStop` already treats a lagged `Done` as expired).

**Abandoned.** A registry entry whose checkout time is older than that budget. Only those sockets are closed, and only from `borrowAfterPoolWait` / `recoverLostTurnsLocked` — the existing pool-wait recovery path. No sweeper goroutine.

**Gap.** The two windows #66 documents: after `borrow` returns and before `doWithHeldSocket` increments `heldSockets`, and after that defer runs and before `release` publishes to idle. A conn in a gap is in the registry with a *young* timestamp. Age, not “not in idle,” is what keeps it unclosed.

**Turn refill vs fd reclaim.** Turn refill stays #66: idle empty and `heldSockets == 0`. Fd reclaim is a second step on that same path: close registry entries older than the budget even when idle is non-empty or a command is in `do`/`dial`. Closing is not gated on the refill predicate.

```
borrow ──register(now)──► doWithHeldSocket (heldSockets++) ──► release ──deregister──► idle
                │                    │                         │
                │                    │                         │
         gap-young               in-do young              gap-young
         (do not close)          (do not close)       (do not close)

panic after borrow: registry keeps the conn. Waiter after PoolTimeout:
  age < budget  → refill turns (heldSockets 0), do not close
  age >= budget → close fd, then refill if idle empty and heldSockets 0
```

## Decisions

- **Registry, not a count.** `heldSockets` cannot close a socket. Add `checkedOut map[*pooledConn]time.Time` under `checkedOutMu` (Yaegi: map + mutex, no generics, no extra goroutine). Register at the end of `takeIdleOrDial` before return (idle reuse and successful dial). Deregister at the start of `release` (both branches). Pointer identity is the key; the same `*pooledConn` is not reused after `close`.
- **Reclaim only on pool-wait.** Hook close into `borrowAfterPoolWait` so a timed-out waiter can close old checkouts *before* the `heldSockets` / idle gates. Do not start a goroutine. Do not close from `Close()` (in-flight commands still finish; spec and ticket: reclaim only on this path).
- **Age bound = command budget, not a multiplier.** Use the mapped `maxRetries` `bindCommandDeadline` already uses (`retryLimits`: New-default 1 → budget 600ms at zero timeouts; `MaxRetries: -1` → one attempt). Predicate: `now.Sub(checkedOutAt) >= budget`. Late is harmless; early is an incident. A live command past a waiter’s `PoolTimeout` (200ms default, shorter in tests) is still younger than 600ms (or the test’s configured budget). Tests must set `IOTimeout` / `DialTimeout` so the in-flight delay is less than the budget and the panic-then-sleep wait is greater.
- **Do not close the held-socket-lease debt.** Young registry entries cannot replace `heldSockets` for *turn refill*. After `bugPanicAfterBorrow`, the conn is registered and young; `heldSockets` is 0. If refill waited for “no young checkouts,” `TestBugLostInUseTurnBricksPoolPermanently` would return `redis:unreachable` instead of restoring turns. Age cannot tell panic-abandoned from gap-live. Leave `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md`. Keep `heldSockets` around `do` and dial.
- **Turn refill stays.** `recoverLostTurnsLocked` still refills when idle is empty and `heldSockets == 0`. `LostTurns()` still counts tokens restored. Spurious refill in the gaps stays the documented bounded overshoot.
- **Counter.** Export `AbandonedClosed() int64` next to `LostTurns()`, incremented once per socket this path closes. Honesty tests read both.
- **Tests (same package, untagged).** (1) `PoolSize` `bugPanicAfterBorrow`, sleep `>=` budget, then Get; `waitOpenSocketsEqual` settles at the live bound (new dials), not `connections()` accepts. (2) `holdGetsForTest` in-flight Get, other goroutine pool-wait; holder completes; its socket stays open; run `-count=5`. (3) Same-package: `borrow` and hold without `do`/`release` (and a sibling that `doWithHeldSocket` then holds before `release`); waiter times out; held conn still usable / still open. (4) `LostTurns()` after panic+Get still `>=` leaked turns; `AbandonedClosed()` stays 0 until age allows close, then matches closed fds.
- **Invariants.** No `defer sr.release` in `exec`. No extra dial on `TestPoolWaitTimesOutWithoutExtraDial`. `TestConnectionIsReused` still 25 Gets / 1 TCP. Do not edit `simpleredis/BUGS.md`. Do not pre-empt #67’s `borrow` 3-tuple; register in `takeIdleOrDial` body.
- **Spec.** Delta on `std_go_simpleredis_tcp-session`: recovered panics must eventually close the abandoned fd; reclaim only from pool-wait; only when checkout age is at least the command budget; live commands and gap conns MUST NOT be closed.
- **Stack.** Base remains `origin/2026-09-13-simpleredis-lost-turn-recovery` (PR #66). #66 must merge first.

## Open questions

- Q: Should this change also close the held-socket-lease debt (drop the bare `heldSockets` count from the refill predicate)?
  Rank: additive asked — ticket Desired names taking that debt if the registry closes the two gaps; the reshape is one extra predicate on `recoverLostTurnsLocked` (1 caller: `borrowAfterPoolWait` in `simpleredis/pool.go`)
  Decision: resolved — leave the debt. Young checkouts after panic would block #66 turn refill; age cannot distinguish gap-live from panic-abandoned. Keep `heldSockets` for do and dial. Reclaim fds by age only.
  By: explore

- Q: What exact duration is the reclaim age bound?
  Rank: additive asked — criterion names `(MaxRetries+1)*(DialTimeout+IOTimeout)` via `bindCommandDeadline`
  Decision: assumed — use that same mapped duration (`retryLimits` then `maxRetries+1` times the two timeouts). No extra multiplier. Tests choose timeouts so live work is under the bound and the abandoned-close wait is over it.
  By: explore

- Q: Name of the new read-only counter besides `LostTurns()`?
  Rank: additive asked — Desired allows a new read-only accessor
  Decision: assumed — `AbandonedClosed() int64`, next to `LostTurns()` / `OverFrees()`. Not a writable field.
  By: explore
