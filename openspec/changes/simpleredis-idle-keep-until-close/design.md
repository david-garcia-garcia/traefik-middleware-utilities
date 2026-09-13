## Context

Dest `takeIdleConn` (`simpleredis/pool.go`) sweeps `idleConns` by `now.Sub(conn.lastUsed) < idleTimeout` and LIFO-pops one survivor. The only caller is `borrow`. `New` starts no goroutine. `Close` drains idle sockets and is idempotent. Dest spec and usage already allow a quiet client to keep sockets until `Close`. Dest product `New` paths do not call `SimpleRedis.Close` (`e2e/simpleredisprobe/plugin.go`; `windowcounter/limiter.go` Close leaves the injected client open). Wiring those callers is out of scope. Yaegi v0.16.1 `interp._select` races when interpreted code selects on a context channel from a goroutine (`simpleredis/resp.go` `watchConnClose`). See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Record the three-direction cost comparison and pick direction 3.
- Leave dest idle-pool contract unchanged.

**Non-Goals:**
- A ticker goroutine in `New`.
- An absolute park-expiry stamp that still only runs on borrow.
- Changing how `borrow` validates or reports a reused socket (BUG-1 sibling).
- `reclaim.Hooks{Close}` or any caller `Close` wiring.
- Rewriting the idle-pool spec or the usage gotcha.

## Decisions

1. **Do not start a background reaper.** Direction 1 is the only design that closes fds while traffic is stopped. Cost: `New` starts a live goroutine (reverses dest spec and `TestStaleIdleHeadIsClosedWhileTailStaysHot`); dest Traefik/`windowcounter` never `Close`, so the ticker leaks on the production path; `Close` must stop the ticker without racing a sweep; a goroutine+`select` stop loop is the Yaegi class `watchConnClose` already avoided. Alternative: ticker plus wiring `Close` on every owner — rejected; those packages are out of scope. Alternative: `time.AfterFunc` per parked socket — still a timer per idle fd, still needs cancel on borrow/`Close`, and forgetting `Close` still leaks.

2. **Do not stamp an absolute expiry at park.** `lastUsed` vs `now` at borrow is already that comparison. An `expiresAt` field would not run during silence, so it does not address the hunt. It would only rename the reuse gate.

3. **Leave dest; treat request-path corpses as BUG-1.** Fd and `maxclients` pressure is bounded by `PoolSize` per plugin instance (default 8). Quiet time turning into guaranteed next-request failures needs a dead socket (restart, `CLIENT KILL`, or a positive Redis `timeout`). Dest compose is `timeout 0`. The sibling stale-pool retry ticket owns borrow validation. Alternative: a permanent untagged test that asserts idle and server-open drop after a multiple of `IdleTimeout` — rejected; that test encodes direction 1 and would fail dest as specified.

## Risks / Trade-offs

- [Quiet Traefik workers keep up to `PoolSize` Redis clients until process death] → Mitigation: already specified; bounded; prior debt `knowledge/debt/2026-09-12-simpleredis-idle-reaper-reclaim.md` remains the follow-up if someone later owns `Close` wiring.
- [Pinned sockets become BUG-1 corpses after restart] → Mitigation: sibling ticket; this change does not edit `borrow`.
- [A later reader treats the hunt FAIL as an unfixed dest bug] → Mitigation: dest spec and usage already say MAY keep until `Close`; this proposal points at that contract.

## Migration Plan

None. No deploy contract. No apply.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
