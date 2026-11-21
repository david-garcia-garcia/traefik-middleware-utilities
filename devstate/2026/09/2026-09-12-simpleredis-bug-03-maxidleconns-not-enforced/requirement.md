# Requirement
IssueKey: 2026-09-12-simpleredis-bug-03-maxidleconns-not-enforced

## Problem
`Config.MaxIdleConns` is documented as the unused-socket keep cap, but `release` only closes when idle is already at that cap **and** `live >= liveCap()`. When `MaxIdleConns < PoolSize`, the socket still holds its in-use turn, so `live` is inflated and the trim almost never fires. Operators who set a small idle cap under a larger `PoolSize` keep nearly a full pool of resting sockets.

## Current (code)
- Contract: `simpleredis/config.go` documents `MaxIdleConns` as how many unused sockets `release` will keep; 0 becomes 8 (`applyDefaults`). `simpleredis/simpleredis.go` `MaxIdleConns()` returns the frozen field.
- Trim: `simpleredis/pool.go` `release` closes only when `closed` or (`len(idleConns) >= maxIdleConns` **and** `len(idleConns)+inUse >= liveCap()`). `inUse` is `liveCap() - len(inUseTurns)` under `idleConnsMu`. `freeInUseTurn` runs after that check, so this socket still counts as in-use.
- `len(inUseTurns)` is sampled under `idleConnsMu`; `borrow` takes a turn without that lock (`simpleredis/pool.go`). Channel len is not a data race; the sample is still unsynchronised with borrowers.
- Live sockets stay bounded by the `inUseTurns` semaphore (`ensureInUseTurns` / `borrow` / `freeInUseTurn` in `simpleredis/pool.go`). Ticket: not an unbounded leak.
- `TestReleaseKeepsSocketWhenLiveUnderCap` in `simpleredis/pool_test.go` asserts idle **exceeds** default `MaxIdleConns` (8) after 12 overlapping Gets with `PoolSize` 16. That pins the current conjunction, not the documented cap.
- Fake server already exposes accepts (`connections`), still-open sockets (`openSockets`), and peer closes (`hangupCount`) in `simpleredis/fake_redis_test.go`.
- go-redis `Put` closes when `idleConnsLen >= MaxIdleConns` even if live is under `PoolSize` (`knowledge/research/ext_go-redis_connection-pool/notes.md`). SimpleRedis does not copy that idle-only trim.

## Desired
- Trim unused sockets on the idle list alone: close when closed or `len(idleConns) >= maxIdleConns`; drop the `live`/`inUse` term from `release`.
- Delete `inUse`/`live` from `release`. Keep `liveCap()` for `PoolSize()`.
- Prove: borrow `PoolSize` sockets, release one by one, assert `len(idleConns) == min(MaxIdleConns, PoolSize)` for table `8/2`, `16/1`, `2/8`, `8/8`. Also quiesce after concurrent commands and assert idle at the cap (peak during the run, not only after). Assert fake-server live sockets (accepts minus closes / `openSockets`) so a fix cannot hide sockets off `idleConns` without closing them.

## Affected
- `simpleredis/pool.go` `release` (and its comment).
- `simpleredis/pool_test.go` `TestReleaseKeepsSocketWhenLiveUnderCap` (will fail if the cap is honoured).
- New/updated tests in `simpleredis/` using `borrow`/`release` (same package; `pool_e2e_test.go` already calls them) and `fakeRedis` socket counts.
- Callers of `release`: `simpleredis/commands_exec.go` `exec`.

## Out of scope
- Idle reaper / `ConnMaxIdleTime` closer (`risk-02-no-idle-reaper.md` and other `simpleredisfixes2/` files).
- Clamping `MaxIdleConns` to `PoolSize` in `applyDefaults`, changing `defaultMaxIdleConns`, or documenting reconnect-vs-footprint tradeoffs (`perf-01-connection-pool-cap.md`).
- Changing `PoolSize` semantics (config comment “idle plus in-use” vs semaphore of in-use turns only).
- go-redis `MaxIdleConns` 0 = unlimited; this package’s 0 = 8.

## Unknowns
- Ticket table (idle 7 for `PoolSize` 8 / `MaxIdleConns` 2) was measured outside this tree; DestBranch code matches the conjunction, not a re-run of that harness.
- Peak-during-run sampler from the audit harness is not in this tree.
- Whether `openSockets()` or `connections()-hangupCount()` is the proof of “accepts minus closes” after close is asynchronous.

## Tensions
- Ticket vs `TestReleaseKeepsSocketWhenLiveUnderCap`: DestBranch test wants idle above `MaxIdleConns` while live is under `PoolSize`; the ticket wants that path to close.
- Ticket vs `release` comment (“full at the live cap”) vs `Config.MaxIdleConns` comment (keep that many unused sockets). Honour the config contract.
- No `RETHINK` comments.
