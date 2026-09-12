# Requirement
IssueKey: 2026-09-12-simpleredis-risk-02-no-idle-reaper

## Problem
SimpleRedis `IdleTimeout` is a lazy reuse gate, not a reaper. `New` starts no goroutine. `takeIdleConn` pops the LIFO tail and returns on the first still-young socket, so older head sockets stay established past idle while traffic recycles the newest one. A fully quiet client never even reaches that gate.

## Current (code)
- `simpleredis/simpleredis.go` `New` — copies `Config`, calls `ensureInUseTurns`, returns. No ticker, no goroutine.
- `simpleredis/pool.go` `takeIdleConn` — pops `idleConns` from the tail; returns on the first `now.Sub(lastUsed) < idleTimeout`; older sockets only go to `stale` if they were at the tail. A young tail leaves the head unexamined.
- `simpleredis/pool.go` `borrow` — closes `stale` after the lock; if reused is nil, dials. Does not sweep the rest of the list.
- `simpleredis/pool.go` `release` — sets `lastUsed` and `append`s to the idle tail. Does not inspect or drop the head.
- `simpleredis/simpleredis.go` `Close` — copies `idleConns`, nils it, closes every remaining socket. That is the only full-list close.
- `simpleredis/config.go` — `IdleTimeout` default 30s (`0` at New becomes that). Comment: how long an idle socket may sit before **borrow refuses to reuse** it.
- `simpleredis/pool_test.go` `TestIdleTimeoutOpensANewConnection` — one idle entry, backdates that sole (tail) `lastUsed`, asserts a second dial. Does not cover a stale head behind a fresh tail. Does not assert `openSockets()` after quiet time with no borrow.
- `simpleredis/fake_redis_test.go` `openSockets` — accept-minus-close count exists; idle-timeout tests do not use it for a no-traffic wait.
- `simpleredis/pool_e2e_test.go` — live Redis/Dragonfly prove pool-wait and peer-close redial (`CLIENT KILL`), not idle-head close after quiet time.
- `windowcounter/limiter.go` `Close` — stops the limiter ticker; comment: does not close the injected SimpleRedis. `tokenbucket/` has no `Close`. `e2e/simpleredisprobe/plugin.go` calls `simpleredis.New` and never `Close`. Product `simpleredis.Close` call sites outside tests: not found.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — an idle conn older than `IdleTimeout` SHALL not be reused. No scenario that the cold head is closed while a younger tail is recycled, or that sockets close after quiet time with no borrow.
- `knowledge/devdocs/std_go_simpleredis.md` — pool knobs and `Close` drains idle; no idle-head reap / no-reaper gotcha.
- `knowledge/research/ext_go-redis_connection-pool/notes.md` — go-redis `ConnMaxIdleTime` is checked lazily on the next `Get` (`isHealthyConn`), not a background reaper. Same lazy shape as dest `takeIdleConn`.

## Desired
- Age out idle sockets past `IdleTimeout` so they do not stay established behind a recycled tail. Ticket prefers **full-list sweep in `takeIdleConn`**: keep LIFO reuse of the newest survivor; close stale outside the lock (already the dest pattern).
- A background reaper is the other option (one goroutine per client, stopped by `Close`). Ticket says it is only viable with reclaim-owned lifecycle (`reclaim.Hooks{Close}`), because dest product code does not call `SimpleRedis.Close`. Sweep does not help a client that never borrows again; if that case matters, combine sweep with a reclaim-owned reaper.
- Prove on the fake: pool *n* sockets, stop traffic, wait past `IdleTimeout`, assert `openSockets()` (accept-minus-close) is 0 — not only `len(idleConns)`. Differing ages: age only the head, borrow once, assert the aged head sockets were closed (a test that only checks the returned conn passes today). `runtime.NumGoroutine()` unchanged across `New` if sweep; back to baseline after `Close` if reaper.

## Affected
- `simpleredis/pool.go` (`takeIdleConn`; possibly `release` / a reaper if explore takes that option)
- `simpleredis/simpleredis.go` (`New` / `Close` only if a reaper is taken)
- `simpleredis/pool_test.go` (quiet-time `openSockets` proof; two-age head close)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (idle-pool scenario — propose work)
- `knowledge/devdocs/std_go_simpleredis.md` after apply (usage gotcha)
- `reclaim/` only if explore takes a reclaim-owned reaper

## Out of scope
- Other `simpleredisfixes2/` files (including bug-03 MaxIdleConns enforcement). Ticket names that overlap; do not take it.
- Changing `IdleTimeout` / `PoolSize` / `MaxIdleConns` defaults.
- Making Redis/Dragonfly server `timeout` a product setting.
- Importing go-redis. EVALSHA, pipelining, TLS, Unix sockets.
- Implementing OPEN PR 12’s peel-on-release as this ticket’s ask (same root cause, different mechanism — tension, not this dump’s desired).

## Unknowns
- Whether a fully idle client (no later `borrow`) must drop sockets, which would need a reclaim-owned reaper in addition to sweep.
- How this ticket relates to OPEN PR 12 (`2026-09-11-simpleredis-perf-03-idle-reaper`, peel stale heads on `release`). Dest `master` does not have that peel. Explore chooses land / wait / diverge.
- Whether idle-list length is still `PoolSize` until bug-03 lands (ticket: sweep cost is bounded by `MaxIdleConns` once that is fixed).

## Tensions
- Ticket prefers full-list sweep on borrow. Sibling OPEN PR 12 implements sweep-on-release (peel heads until a still-valid head). Same root cause; dest master still has tail-only `takeIdleConn`.
- Ticket also offers a background reaper. Dest has no ticker in SimpleRedis; `Limiter.Close` does not close SimpleRedis; a reaper without reclaim Close leaks goroutines. Ticket says reclaim is the owner if that option is taken.
- tcp-session spec today only forbids **reuse** of a stale idle conn. Closing the cold head (and closing after quiet time with no borrow) is the finding; a spec scenario is propose work, not a new product ask.
- Ticket’s proof recipe is fake-server `openSockets` plus a two-age borrow. It does not ask for live Redis/Dragonfly idle-head proof. PR 12 added live idle-head tests; that is PR 12’s ask, not this dump’s.
