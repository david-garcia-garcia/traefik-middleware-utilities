# Explore
IssueKey: 2026-09-12-simpleredis-bug-03-maxidleconns-not-enforced

## Concepts

```
borrow (takes in-use turn) ──► command ──► release
                                              │
                    len(idle) < maxIdleConns ─┼──► keep on idleConns, free turn
                    idle already full       ─┴──► close socket, free turn
```

`MaxIdleConns` is the unused-socket keep cap (`simpleredis/config.go`). `PoolSize` / `liveCap()` is the in-use-turn semaphore (`simpleredis/pool.go`). Idle sockets do not hold a turn (`simpleredis/simpleredis.go`). go-redis `Put` already closes when idle is full even while live is under `PoolSize` (`knowledge/research/ext_go-redis_connection-pool/notes.md`). SimpleRedis `release` currently ANDs idle-full with `live >= liveCap()`, and `live` still counts this socket because `freeInUseTurn` runs after the check.

The live spec `std_go_simpleredis_tcp-session` already says unused sockets SHALL be at most `MaxIdleConns`, then also says `release` MUST NOT close a reusable socket solely because idle is full while live is under `PoolSize`. Config comment and the first SHALL win; the MUST NOT sentence is the DestBranch conjunction this ticket reverses.

Production caller of `release`: `simpleredis/commands_exec.go` `exec` only. Tests in `simpleredis/pool_test.go` (`TestReleaseKeepsSocketWhenLiveUnderCap`) pin idle **above** default 8 after 12 overlapping Gets with `PoolSize` 16. That test passed on this worktree (`go test ./simpleredis -run TestReleaseKeepsSocketWhenLiveUnderCap`).

Host identity is not reconstructed here (no client address / user / tenant / Host / trust hop).

## Decisions

- Trim on the idle list alone: close when `closed` or `len(idleConns) >= maxIdleConns`. Delete `inUse` / `live` from `release`. Keep `liveCap()` for `PoolSize()`.
- Honour `Config.MaxIdleConns` and the spec’s “at most MaxIdleConns” sentence. Delta the MUST NOT-close-under-`PoolSize` sentence on `std_go_simpleredis_tcp-session`. Fold, do not add a new spec leaf.
- Replace `TestReleaseKeepsSocketWhenLiveUnderCap` so it no longer requires idle above the cap. Sequential `borrow`/`release` table `8/2`, `16/1`, `2/8`, `8/8` asserts `len(idleConns) == min(MaxIdleConns, PoolSize)`. Concurrent commands: after Wait, idle at the cap. Fake live sockets via `waitOpenSocketsEqual` so extras are closed, not hidden off `idleConns`.
- Keep a reuse Get that does not dial when idle is at the cap (the last third of the current test).
- Out of scope stays out: idle reaper, clamping `MaxIdleConns` to `PoolSize`, `PoolSize` semantics, go-redis `0` = unlimited.

Throwaway `borrow`/`release` on DestBranch (deleted after; not in the product tree):

| PoolSize | MaxIdleConns | idleTrace | Final idle | Want |
| --- | --- | --- | --- | --- |
| 8 | 2 | 1 2 2 3 4 5 6 7 | 7 | 2 |
| 16 | 1 | 1 then 1..15 | 15 | 1 |
| 2 | 8 | 1 2 | 2 | 2 |
| 8 | 8 | 1..8 | 8 | 8 |

The 8/2 trace matches the ticket table. `TestReleaseKeepsSocketWhenLiveUnderCap` still passes — DestBranch pins the gap.

Usage `knowledge/devdocs/std_go_simpleredis.md` already says idle trim is `MaxIdleConns`. Language needs no new term. Research folder already answers go-redis `Put`. No new packet this phase.

## Open questions

- Q: Does DestBranch still grow idle past MaxIdleConns for PoolSize 8 / MaxIdleConns 2 as the ticket table (idle 7)?
  Rank: additive asked — measurement of current behavior; requirement Desired names the trim
  Decision: resolved — throwaway sequential borrow/release printed idleTrace `[1 2 2 3 4 5 6 7]`, final idle 7, want 2. Same conjunction as `pool.go` `release`.
  By: explore

- Q: How to prove peak idle during a concurrent run, not only after quiesce, without the missing audit-harness sampler?
  Rank: additive asked — requirement Desired asks peak during the run; no new sampler in this tree
  Decision: assumed — sequential table records idle after each `release` (that series is the peak). Concurrent path asserts after `Wait` plus `waitOpenSocketsEqual` at `min(MaxIdleConns, PoolSize)`. Do not import the external harness.
  By: explore

- Q: Is fake live-socket proof `openSockets()` or `connections()-hangupCount()` after close is asynchronous?
  Rank: additive asked — requirement Desired asks accepts-minus-closes / openSockets
  Decision: assumed — use existing `waitOpenSocketsEqual` (`fake_redis_test.go`). `hangupCount` increments then `open--` in `serve` defer; a snapshot can see hangups before open drops (`8/2` throwaway: hangups=1, open=8). `connections()-hangupCount()` is not still-open. Wait on `openSockets`.
  By: explore

- Q: Who already owns client address, user, tenant, Host, or trust hop on this path?
  Rank: additive asked — explore identity gate; this change does not set those facts
  Decision: resolved — none. `release` closes or keeps a pooled TCP socket; it does not reconstruct identity. No owner to reuse.
  By: explore
