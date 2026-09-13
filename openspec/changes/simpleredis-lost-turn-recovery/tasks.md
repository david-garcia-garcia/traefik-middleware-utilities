## 1. Turn recovery

- [x] 1.1 Add `heldSockets` and `lostTurns` atomics on `SimpleRedis`; export `LostTurns() int64` next to `OverFrees()`
- [x] 1.2 Increment `heldSockets` around `dial` inside `borrow`; decrement on dial error after the turn is freed
- [x] 1.3 Increment `heldSockets` in `exec` after a successful `borrow` with `defer` decrement; do not `defer sr.release`
- [x] 1.4 When `borrow` would return `errPoolWait`, if idle is empty and `heldSockets` is 0, refill `inUseTurns` to `cap`, add the restored count to `LostTurns()`, then take a turn and proceed
- [x] 1.5 Keep recovery from firing when sockets are busy; keep `freeInUseTurn` over-free behavior

## 2. Tests

- [x] 2.1 Port `TestBugLostInUseTurnBricksPoolPermanently` and `bugPanicAfterBorrow` into the default suite untagged so the test PASSES after the fix
- [x] 2.2 Busy-pool test: saturated in-flight commands still cap at `PoolSize`, still return pool-wait `redis:unreachable`, never dial past the cap, `LostTurns()==0`
- [x] 2.3 `LostTurns()` reports the leak after recovered panics; balanced Get leaves `LostTurns()==0`
- [x] 2.4 Keep green: `TestOverFreeAccountingStaysBalanced`, `TestPoolWaitTimesOutWithoutExtraDial`, `TestConcurrentCommandsStayWithinPool`, `TestBurstGetsStayWithinLiveCap`, `TestOverlappingCallersDoNotDialPastLiveCap`, `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestGetCancelWhileWaitingForTurn`, `TestOverFreeOnFullSemaphoreReturns`
- [x] 2.5 `go vet ./simpleredis/` and `go test -count=1 -timeout 300s ./simpleredis/`
- [x] 2.6 Repeat pool tests `-count=5` (no local `-race`; no C toolchain)

## 3. Usage packet and spec

- [x] 3.1 Gotcha on `knowledge/devdocs/std_go_simpleredis.md`: leaked turns refill at pool-wait when owned sockets are zero; `LostTurns()` diagnoses a bricked-then-recovered client
- [x] 3.2 Confirm the change delta `std_go_simpleredis_tcp-session` matches the landed tests
- [x] 3.3 Run `openspec validate --change simpleredis-lost-turn-recovery --strict`
