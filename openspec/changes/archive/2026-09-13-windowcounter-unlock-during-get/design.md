## Context

Dest `takeBuffered` / `peekBuffered` / `flushPending` lock `l.mu` then `defer Unlock` and call `getCount` / `Eval` before Unlock (`windowcounter/limiter.go`). Dest fake GET replies immediately. Parent checkout already has `blockGetPrefix` / `waitGetBlocked` / `unblockGet` and `repro_lock_during_get_test.go`. See proposal.md for why. Specs: `std_go_windowcounter_sliding-take`, `std_go_windowcounter_sync-flush`. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Tests first: port GET-hold helpers, land the HOLB repro, confirm FAIL, then unlock-around-I/O, then PASS.
- No Redis GET or Eval while `l.mu` is held on buffered paths, including outage probe.
- Fast Take finishes well under a 400ms GET delay on another key.
- After Eval success, `localDelta -= flushedDelta`.

**Non-Goals:**
- Bug 2: skip GET on an already-mapped previous window.
- Other `windowcounter/BUGS.md` items on the parent checkout.
- Changing exact-mode INCR/EXPIRE/GET.
- Rewriting `flushScript`.
- Renaming `*Locked` helpers.

## Decisions

1. **Port GET-hold onto dest `testFakeRedis`, then add `windowcounter/repro_lock_during_get_test.go` before any limiter change.** `serve` MUST Unlock `f.mu` during the hold so the fake does not stall every key. Alternative: sleep inside GET while holding `f.mu` — rejected; that is a different HOLB and would not prove the limiter mutex.

2. **Snapshot, unlock, Redis, re-lock, merge.** `takeBuffered` / `peekBuffered` MUST NOT `defer Unlock` across GET. First-sight current (`windowLocked` miss), `localDelta == 0` current-key GET, first-sight previous (`bufferedCountLocked` miss), and first-sight Peek GET all use this. Alternative: per-key mutex — rejected; ticket named unlock-around-I/O on `l.mu`. Alternative: skip the `localDelta == 0` current GET — rejected; that GET is this HOLB, not bug 2.

3. **Merge on re-lock:** if the key is missing, insert `{redisKnown: known, expireAt}`. If present, keep `localDelta`; set `redisKnown` from this GET only when `localDelta == 0`. Alternative: always overwrite `redisKnown` — rejected; a concurrent Take's unflushed hits would be dropped if the GET was stale.

4. **Flush: copy `(key, delta, expireAt)` under lock, mark that delta in-flight, unlock, Eval, re-lock.** Success: `redisKnown = n`, `localDelta -= flushedDelta`, clear in-flight. Failure: clear in-flight, leave `localDelta`. A second flusher skips a key whose snapshot is in-flight. Alternative: `localDelta = 0` after Eval — rejected; ticket forbids that when a Take landed during Eval.

5. **`bufferedOutageErrorLocked` nested flush and probe GET use the same unlock-around-I/O.** Call sites remain `flushLoop`, `Sleep`, `Close`, and the outage probe (all `windowcounter/limiter.go`). Alternative: leave the probe under `l.mu` — rejected; Desired #2 is no Redis under the mutex.

6. **Keep helper names.** Unlock lives inside the existing helpers or their callers; do not rename `windowLocked` / `flushPendingLocked`. Alternative: rename to drop `Locked` — rejected; Bound the ask.

## Risks / Trade-offs

- [Two Takes first-sight the same key and both GET] → Mitigation: merge keeps `localDelta`; apply GET `redisKnown` only when `localDelta == 0`.
- [Two flushers Eval the same delta] → Mitigation: in-flight mark; second flusher skips that key.
- [Stale GET after a successful flush] → Mitigation: apply GET `redisKnown` only when `localDelta == 0`; a just-flushed window has `localDelta` reduced, not blindly zeroed past concurrent hits.
- [Outage probe still serializes if it re-takes `l.mu` around many keys] → Mitigation: copy one key, unlock, GET, re-lock; same as first-sight.

## Migration Plan

No signature change. Rollback is revert. Unit proof is `go test -short -count=1 -timeout 60s ./windowcounter`.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
