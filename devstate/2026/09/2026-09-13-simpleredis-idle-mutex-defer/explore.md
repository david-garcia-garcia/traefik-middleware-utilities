# Explore
IssueKey: 2026-09-13-simpleredis-idle-mutex-defer

## Concepts

```
  dest release (reusable path)
  ─────────────────────────────
  lastUsed = now
  idleConnsMu.Lock()
    decide keep-or-close          ← panic here
    if close: Unlock; conn.close(); freeInUseTurn; return
    append idle
  Unlock
  freeInUseTurn
```

`idleConnsMu` guards the unused-socket list. Dest `takeIdleConn` already `Lock` + `defer Unlock` and returns stale sockets for close after the lock. Dest `Close` already swaps the slice under the lock and closes sockets outside it. Dest `release` is the leftover: two hand `Unlock`s, then `conn.close()` / `freeInUseTurn` after.

A panic between `Lock` and either `Unlock` leaves the mutex locked for the process lifetime. Traefik recovers the request. Later `borrow` waits in `takeIdleConn`, later `release` waits on the same mutex, `Close` waits. Stuck goroutines accumulate. That is worse than the in-use-turn leak #71 already defers around.

#71 (dest) already defers `release` in `runOnConn` and `freeInUseTurn` in `borrow`. One call site of `release`: `commands_exec.go` `runOnConn`. `resp.go` `do` comment and the `pull/29` comment are gone. `yaegi_defer_test.go` is the permanent proof that interpreted `defer` runs on those three panic classes. `knowledge/devdocs/std_go_simpleredis.md` and `openspec/specs/std_go_simpleredis_tcp-session/spec.md` already record that finding. No stale "defer does not run" sentence in those two.

`simpleredis/BUGS.md` section 2 still says `exec` releases without `defer` and names `2026-09-13-simpleredis-lost-turn-recovery`. The ticket quote "Do not simply switch to `defer sr.release(...)`; PR 29 rejected that…" is not on dest. Section 2 is still the file that will send the next reader down the wrong path.

Usage packet `knowledge/devdocs/std_go_simpleredis.md` is enough to call the subsystem. The mutex extract is package-internal; Language has no gap. Do not produce a new packet. Yaegi research folders already exist; this ticket does not depend on a new third-party fact.

In-flight `2026-09-13-simpleredis-close-abandoned-socket` `release` is dest's body plus `sr.deregisterCheckout(conn)` at the top. Mechanical conflict on this function. Document; do not wait; do not take that branch's checkout table.

## Decisions

- Extract the locked keep-or-close decision into `parkIdleConn` (sibling of dest `takeIdleConn`). That method `Lock`s, `defer`s `Unlock`, computes `idleConnsFull` / `inUse` / `live`, appends on success, returns bool (parked). `release` acts after it returns.
- `release` shape: stamp `conn.lastUsed` only on the reusable path (before `parkIdleConn`, including when the later verdict is close). Then `if !reusable || !sr.parkIdleConn(conn) { conn.close() }`. Then `sr.freeInUseTurn()` last on every path. Two `conn.close()` sites become one; three `freeInUseTurn()` sites become one.
- Do not `defer sr.idleConnsMu.Unlock()` at the top of `release`. Unlock must not span `conn.close()` or the `freeInUseTurn` channel send. Dest `Close` and `takeIdleConn` already have the correct "work under the lock, close fds after" idiom; leave them.
- Close decision unchanged: close when the client is closed, OR when `idleConns` is already at `maxIdleConns` AND `live >= liveCap()`. Otherwise park. `inUse = liveCap() - len(inUseTurns)` still runs while this socket's turn is held.
- `OverFrees` unchanged: exactly one `freeInUseTurn` per `release`. `parkIdleConn` MUST NOT call `freeInUseTurn`.
- No production hooks or seams to panic inside the critical section. No new panic-inside-`parkIdleConn` test. Existing `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestOverFreeOnFullSemaphoreReturns`, `TestGetCancelFreesTurnAndDoesNotPool`, `TestPanicInDoReturnsTurnAndClosesSocket`, and the concurrent pool tests cover the four semantics without an API change.
- Task B: `cachedGroupWrite` / `storeGroupWrite` convert the two manual `groupWriteMu` unlocks to `defer`. Two-line consistency. Do not restyle the rest of `commands_msetex.go`.
- Task C: rewrite `simpleredis/BUGS.md` section 2 to the measured finding (interpreted `defer` runs on panic under Yaegi v0.16.1 for explicit panic, interpreter `errors.As`, nil-map write) and point at `simpleredis/yaegi_defer_test.go`. Do not restate the probe. Leave `knowledge/devdocs/std_go_simpleredis.md` and the tcp-session spec unless a stale sentence reappears (none on dest).
- Propose a delta on existing `std_go_simpleredis_tcp-session` (FindSpecHost fold): idle-list mutex unlock is owned by `defer` inside the keep-or-close owner; `conn.close` and `freeInUseTurn` run after that method returns. Do not add a new spec folder. Do not change Close / takeIdleConn / `errors.Is` / AfterFunc / `math/rand` / `OverFrees` itself.
- Stacked on #71. DestBranch stays `2026-09-13-simpleredis-panic-safe-release`. Merge after #71. Do not rebase onto master while #71 is open.
- Document in-flight conflicts; do not narrow: `2026-09-13-simpleredis-close-abandoned-socket` (`release` + `deregisterCheckout`); `2026-09-13-simpleredis-desync-boundary-check` (`resp.go`); `2026-09-13-simpleredis-resilience-test-coverage`; `2026-09-13-golangci-lint-harden` (named results on `dial` / `borrow` in `pool.go`).

Not reproduced as a live panic: dest has no seam to panic between `idleConnsMu.Lock` and `Unlock` without a production hook (forbidden). The failure mode is Go mutex semantics plus dest `simpleredis/pool.go` `release` (two hand unlocks at the closed-or-full return and the park path). `liveCap()` is a cap/len read; the panic window is small, the deadlock if it hits is process-lifetime.

## Open questions

- Q: Can a panic-inside-`parkIdleConn` test be written with no production API change?
  Rank: additive asked — requirement Unknowns line; no existing caller reshape
  Decision: assumed — no. Existing pool reuse, live-under-cap, OverFrees, cancel-close, and panic-in-`do` tests cover the four semantics. A panic inside the locked section would need a production hook, which the requirement forbids. Structural value is the extract.
  By: explore
