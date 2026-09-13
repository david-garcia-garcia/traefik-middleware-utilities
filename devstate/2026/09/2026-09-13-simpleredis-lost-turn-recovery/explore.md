# Explore
IssueKey: 2026-09-13-simpleredis-lost-turn-recovery

## Concepts

```
  exec                         pool
    |                            |
    |  borrow: take inUseTurns   |
    |--------------------------->|
    |  idle or dial              |
    |<---------------------------|
    |  do(...)                   |
    |  PANIC ..................  |  release never runs
    |  Traefik recover           |
    |                            |
    |  inUseTurns permanently -1 |
    |  idle empty, no reaper     |
    |  after PoolSize: dead      |
```

- **in-use turn**: one slot in `inUseTurns` (buffered to `liveCap()` / `PoolSize`). Taken in `borrow`, returned in `freeInUseTurn` from `release` (and borrow's own error paths). Idle sockets do not hold a turn.
- **live sockets the pool owns**: unused sockets in `idleConns` plus sockets a still-running command has checked out. After a recovered panic the TCP fd may still be open in the process, but it is not on either list — that is a leaked fd, not a live pool socket.
- **pool wait**: empty `inUseTurns` after `PoolTimeout` → `errPoolWait` (`redis:unreachable`). Spec: that wait is backpressure only when live sockets are actually at `PoolSize`. DestBranch treats an empty channel as that cap even when idle is 0 and nothing is in flight.
- **OverFrees**: extra *returns* when the channel is already full. Cannot restore a missing return.
- **PR 29**: closed, not merged. Owner discarded deferred `release` on panic-unwind as too much complexity for Yaegi recovering the request while the turn stays lost. A pointer comment next to `sr.do` is the rejected approach. This run does not reverse that.

Usage packet `knowledge/devdocs/std_go_simpleredis.md` already names the live cap, pool wait, and `OverFrees()`. It does not mention lost-turn recovery or `LostTurns()`. Research `ext_go-redis_connection-pool` matches DestBranch: Get takes a semaphore, Put releases it; no refill of a lost acquire.

## Decisions

- Follow PR 29: do **not** `defer sr.release(...)` in `exec`. Fix permanence at `borrow` when a pool-wait is about to be returned.
- Cheapest sufficient recovery: when `borrow` would return `errPoolWait`, if owned live sockets are zero, refill `inUseTurns` to `cap` and add the number of tokens restored to `LostTurns()`. Then take a turn and proceed (idle or dial). Do not start a reaper (`New` must not).
- Distinguish "no live sockets" from "all live sockets busy" with an atomic `heldSockets` (name in Open questions) that tracks sockets a *still-running* command holds, plus `len(idleConns)` under `idleConnsMu`. Busy Get/exec increments around `do` with `defer` so Traefik unwind restores the counter even though `release` is skipped. Recovery fires only when both are zero. A saturated pool with in-flight commands still returns `errPoolWait` and must not dial past `PoolSize`.
- `LostTurns()` is a read-only atomic next to `OverFrees()` in `simpleredis.go`. It counts tokens this client has refilled, not a snapshot of `cap-len`.
- Port `TestBugLostInUseTurnBricksPoolPermanently` (and `bugPanicAfterBorrow`) into the default suite untagged. Keep `simpleredis/BUGS.md` untouched. Do not touch `resp.go` or `handshakeFailure` / `shouldRetry`.
- Spec delta on `std_go_simpleredis_tcp-session`: wait-not-dial is backpressure only when the client owns live sockets at `PoolSize`; a leaked turn with zero owned sockets SHALL refill rather than brick. `LostTurns()` readable next to `OverFrees()`.

Reproduced (not implemented): `go test -tags bugrepro -count=1 -timeout 120s -run TestBugLostInUseTurnBricksPoolPermanently ./simpleredis/` from `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` on this DestBranch code:

```
Get after 2 lost in-use turns = redis:unreachable, want success: turns=0/2, idle=0, accepts=2
```

File was not left in the tree.

## Open questions

- Q: Refill missing turns at `errPoolWait` when owned live sockets are zero, or make the turn a leased resource with a reaper?
  Rank: additive asked — new recovery path this change creates; Desired names cheapest sufficient if it preserves the live-socket cap
  Decision: assumed — refill at `errPoolWait` when `len(idleConns)==0` and `heldSockets==0`; no lease object and no goroutine reaper (`New` MUST NOT start one).
  By: explore

- Q: Where is the dialed-but-not-yet-released counter incremented so a recovered panic looks like zero owned sockets and a busy Get does not?
  Rank: additive asked — new field this change creates; Desired requires the no-live vs all-busy distinction
  Decision: assumed — `heldSockets` atomic on the client. Increment in `exec` after a successful `borrow` with `defer` decrement (Traefik recover runs that defer; `release` is still not deferred). Also increment around `dial` inside `borrow` so a waiter that times out during `DialContext` does not refill. Do not increment at turn-take and leave the matching decrement only in `release`: that leaks the same way as the turn and would make the regression (`borrow` then `panic` without `exec`) look busy forever. `bugPanicAfterBorrow` never enters `exec`, so `heldSockets` stays 0 and refill is correct for that test.
  By: explore

- Q: Does the nanosecond window after taking a turn and before `heldSockets` increments (idle reuse returning into `exec`) let recovery fire and dial past `PoolSize`?
  Rank: additive incidental — means to the live-cap invariant; no Desired line names this window
  Decision: assumed — do not add a mutex or reaper for that window. A holder must remain in it for a full `PoolTimeout` (default 200ms) before a waiter can refill; that is not a realistic schedule. Hung TCP is covered by the `dial` increment.
  By: explore

- Q: Should this change also close TCP fds left behind when `borrow` returns a conn that is never `release`d?
  Rank: additive incidental — Desired is turn permanence and `LostTurns()`; leaked fds are not an acceptance line
  Decision: assumed — do not track or close those fds. Refill lets the next command dial. The fd leak after a recovered panic stays as on DestBranch. Not a `knowledge/debt/` note this run unless implement finds a cheap owner already on `pooledConn`.
  By: explore
