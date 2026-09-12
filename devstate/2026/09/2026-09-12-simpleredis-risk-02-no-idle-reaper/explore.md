# Explore
## Concepts
SimpleRedis idle is a LIFO slice (`idleConns`). `release` appends; `takeIdleConn` pops the tail and, on dest `master`, **returns on the first still-young socket**. Older entries at the head are not examined while the newest tail keeps getting recycled. `IdleTimeout` is that reuse gate, not a reaper: `New` starts no ticker (`runtime.NumGoroutine()` unchanged). `Close` is the only full-list close, and product callers outside tests do not call it (`windowcounter/limiter.go` `Close` stops the limiter ticker only; `e2e/simpleredisprobe/plugin.go` never `Close`s).

OPEN PR 12 (`2026-09-11-simpleredis-perf-03-idle-reaper`, not merged into `master`) peels stale **heads on `release`**, stopping at the first still-valid head. `takeIdleConn` on that branch stays tail-and-return. PR 12 itself records the residual: a borrow that happens after timeout but **before** a later `release` can still pop a cold head (or, with dest’s tail-return, skip the head entirely and leave it established).

This ticket’s preferred fix is a **full-list sweep in `takeIdleConn`**: partition idle into survivors vs stale, reuse the newest survivor (LIFO), close stale after the lock (already dest’s `borrow` pattern). go-redis `ConnMaxIdleTime` is the same lazy-on-Get shape (`knowledge/research/ext_go-redis_connection-pool/notes.md`); dest copies that laziness and currently fails to walk the list.

```
idleConns  [oldest head ........ newest tail]
dest takeIdleConn: pop tail → young? return (head never seen)
PR 12 release: peel head prefix until a young head, then append
this ticket: on borrow, walk all, close stale, pop newest survivor
```

Usage packet `knowledge/devdocs/std_go_simpleredis.md` already exists; after apply it needs a gotcha that borrow sweeps stale idle (not a new packet). tcp-session today only forbids **reuse** of a stale idle conn; closing the cold head is propose work on that same spec.

## Decisions
- Land **full-list sweep in `takeIdleConn`** on this branch. PR 12’s peel-on-release does **not** satisfy this dump’s two-age borrow proof (age only the head, borrow once, assert those sockets closed via `openSockets`). Dest `master` still has tail-only `takeIdleConn`. Do not copy PR 12’s peel into this change (`requirement.md` Out of scope).
- Do **not** add a background reaper or `reclaim.Hooks{Close}` in this change. Sweep does not drop sockets on a fully quiet client; that case is a follow-up note.
- Proof on the fake: two-age idle (age only the head), one `borrow`/`Get`, `waitOpenSocketsEqual` for the closed heads — not `len(idleConns)` alone. `runtime.NumGoroutine()` unchanged across `New`. Do not add PR 12’s live Redis/Dragonfly idle-head tests.
- Do not take bug-03 (`MaxIdleConns` only closes when live is also at `liveCap()` — `TestReleaseKeepsSocketWhenLiveUnderCap`). Sweep cost stays bounded by idle length, which dest still allows above `MaxIdleConns` when `PoolSize` is larger. Default `PoolSize` is 8.

## Open questions
- Q: Does OPEN PR 12’s peel-on-release already satisfy this ticket’s desired behavior?
  Rank: bounded asked — one call site (`simpleredis/pool.go` `takeIdleConn`); requirement Desired names full-list sweep in `takeIdleConn` and Out of scope names PR 12’s peel
  Decision: resolved — mechanisms differ. PR 12 peels heads on `release` and leaves tail-return `takeIdleConn`; a two-age borrow with no later release still leaves the stale head established. Implement this finding’s sweep on this branch. Dest is `master`; PR 12 is not merged.
  By: explore

- Q: Must a fully idle client (no later `borrow`) drop sockets after `IdleTimeout`?
  Rank: additive incidental — a reaper goroutine would be new; requirement prefers sweep; quiet-time `openSockets()==0` with no borrow is the reaper measurement, not the tail-only defect
  Decision: assumed — no. Sweep on the next borrow. Quiet-time close without traffic needs a reclaim-owned reaper because dest product code does not call `SimpleRedis.Close`. Noted, not built.
  By: explore

- Q: Is idle-list length still `PoolSize` until bug-03 lands, so sweep is O(PoolSize) rather than O(MaxIdleConns)?
  Rank: additive asked — requirement Unknowns; Out of scope says do not take bug-03
  Decision: resolved — dest `release` still keeps idle above `MaxIdleConns` when live is under `liveCap()` (`TestReleaseKeepsSocketWhenLiveUnderCap`, `pool.go` `idleConnsFull && live >= liveCap()`). Sweep walks whatever idle exists. Do not take bug-03.
  By: explore
