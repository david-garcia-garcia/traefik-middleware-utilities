# Explore
IssueKey: 2026-09-13-windowcounter-bug-lock-during-get

## Concepts

Buffered Take/Peek and the flush ticker share one `sync.Mutex` (`l.mu`) over the whole `windows` map. Dest `takeBuffered` / `peekBuffered` / `flushPending` lock, `defer Unlock`, then call helpers that `GET` or `Eval` before Unlock (`windowcounter/limiter.go`). Exact mode (`sync_rate == 0`) never takes `l.mu`.

HOLB: a first-sight (or `localDelta == 0` current-key) GET on opaque key `slow` holds `l.mu` for the Redis RTT. An unrelated Take on `fast` waits on that same mutex. Dest fake GET replies immediately (`windowcounter/fake_redis_test.go` `serve`); dest has no `blockGetPrefix` / `waitGetBlocked` / `unblockGet`. Parent checkout has those helpers and `windowcounter/repro_lock_during_get_test.go` (400ms GET delay; fail if fast Take waits > 200ms).

Unlock-around-I/O (agreed): snapshot under lock, unlock, Redis, re-lock, apply. Flush copies `(key, delta, expireAt)`, Eval unlocked, then `redisKnown = n` and `localDelta -= flushedDelta` (not `localDelta = 0`).

```
Dest (HOLB)                         Agreed
Take(slow) lock                     Take(slow) lock; snapshot; unlock
  GET slow  ----400ms----             GET slow  ----400ms----
  Take(fast) waits l.mu               Take(fast) lock; admit; unlock
Unlock after GET                    Take(slow) re-lock; apply redisKnown
```

Usage `knowledge/devdocs/std_go_windowcounter.md` documents first-sight GET and buffered Peek; it does not say Take must not hold the limiter mutex across Redis. Specs `std_go_windowcounter_sliding-take` / `std_go_windowcounter_sync-flush` do not mention mutex-across-I/O. Sync-flush says after a successful flush `local_delta` SHALL clear — that letter conflicts with `localDelta -= flushedDelta` when a concurrent Take incremented during Eval. Ticket wins; spec catches up in propose.

Kong research (`ext_kong_rate-limiting_sliding-sync`) is flush semantics, not this mutex. No new research write.

## Decisions

- Tests first, then unlock-around-I/O, then prove the repro passes and `go test -short -count=1 -timeout 60s ./windowcounter` passes.
- Repro file: `windowcounter/repro_lock_during_get_test.go`. Port GET-hold helpers into dest `windowcounter/fake_redis_test.go`. Fake `serve` MUST drop `f.mu` during the GET hold so the fake does not stall every key.
- Do not call Redis (`GET` / `Eval`) while holding `l.mu`. Exact mode unchanged.
- `windowLocked` first-sight GET and `windowLocked` GET when `localDelta == 0` on an already-mapped current key: both unlock around GET (this HOLB, not bug 2).
- Bug 2 (skip GET on an already-mapped previous key) stays out of scope.
- After GET, re-lock and apply `redisKnown`; merge if another Take inserted.
- `flushPending`: copy `(key, delta, expireAt)` under lock, unlock, Eval, re-lock: `redisKnown = n`, `localDelta -= flushedDelta`.
- Keep helper names (`windowLocked`, `flushPendingLocked`); do not rename the neighborhood.

## Open questions

- Q: Does first-sight previous GET in `bufferedCountLocked` use the same snapshot / unlock / merge as `windowLocked`?
  Rank: additive asked — Desired #2 (no Redis while holding `l.mu`); dest `bufferedCountLocked` GETs on map miss under the Take lock (`windowcounter/limiter.go`); Out of scope is bug 2's already-mapped skip, not this first-sight GET
  Decision: resolved — same snapshot, unlock, getCount, re-lock, merge. Already-mapped previous stays a memory read (bug 2 not taken).
  By: implement

- Q: Do `bufferedOutageErrorLocked`'s nested flush and probe GET also drop `l.mu` before Redis?
  Rank: additive asked — Desired #2 general rule; ticket names `flushPending` copy/unlock/Eval; dest probe GET and `flushPendingLocked` run under the Take lock (`windowcounter/limiter.go` `bufferedOutageErrorLocked`)
  Decision: resolved — nested flush uses the same copy/unlock/Eval/re-lock as `flushPending`; probe GET snapshots the key, unlocks, getCount, re-locks. Call sites of `flushPending` / `flushPendingLocked` in this package: `flushLoop`, `Sleep`, `Close`, `bufferedOutageErrorLocked` (four, all `windowcounter/limiter.go`).
  By: implement

- Q: After Eval, how do we apply `localDelta -= flushedDelta` if a second flush copied overlapping delta while the first Eval was in flight?
  Rank: bounded asked — Desired #4 changes dest `flushPendingLocked` which zeros `localDelta` under lock; four call sites enumerated above
  Decision: resolved — mark the copied delta in-flight on that window so a second flusher does not Eval the same snapshot; on Eval success `redisKnown = n` and `localDelta -= flushedDelta`; on Eval failure clear in-flight and leave `localDelta` (including Takes during the round trip).
  By: implement

- Q: What does merge mean when another Take inserted the key during GET?
  Rank: additive asked — Desired #3 names merge if another Take inserted
  Decision: resolved — on re-lock, if the key is missing, insert `{redisKnown: known, expireAt}`. If present, keep that entry's `localDelta`; set `redisKnown` from this GET only when `localDelta == 0`. Do not replace the pointer and drop concurrent hits.
  By: implement

- Q: Test file name on dest?
  Rank: additive asked — Desired #1 example path `windowcounter/repro_lock_during_get_test.go`
  Decision: resolved — use that path.
  By: explore
