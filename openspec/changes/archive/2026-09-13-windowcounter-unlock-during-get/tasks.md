## 1. Failing repro first

- [x] 1.1 Port `blockGetPrefix` / `waitGetBlocked` / `unblockGet` and GET-hold fields into dest `windowcounter/fake_redis_test.go`. `serve` MUST drop `f.mu` during the hold.
- [x] 1.2 Add `windowcounter/repro_lock_during_get_test.go` (opaque keys `slow` / `fast`, 400ms GET hold). Do not change `limiter.go` yet.
- [x] 1.3 Run that test and confirm FAIL (~400ms wait on fast Take).

## 2. Unlock around Redis

- [x] 2.1 `windowLocked` / first-sight Peek GET / first-sight previous GET: snapshot under `l.mu`, unlock, `getCount`, re-lock, apply `redisKnown` (insert if missing; if present keep `localDelta`, set `redisKnown` from GET only when `localDelta == 0`). Include the dest `localDelta == 0` current-key GET.
- [x] 2.2 `flushPending`: copy `(key, delta, expireAt)` under lock, mark in-flight, unlock, Eval, re-lock: `redisKnown = n`, `localDelta -= flushedDelta`, clear in-flight. On Eval failure clear in-flight and leave `localDelta`. Skip a key whose snapshot is already in-flight.
- [x] 2.3 Outage probe nested flush and probe GET use the same unlock-around-I/O. Exact mode unchanged.

## 3. Prove

- [x] 3.1 Confirm `TestRepro_BufferedTakeHoldsLockDuringRedisGet` PASSES.
- [x] 3.2 Confirm `go test -short -count=1 -timeout 60s ./windowcounter` passes.
