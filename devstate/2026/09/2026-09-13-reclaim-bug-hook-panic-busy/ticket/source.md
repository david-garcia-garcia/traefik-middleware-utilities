# Panic or nil create leaves the key slotBusy forever

Bug: panic or nil create leaves the key slotBusy forever (later Open hangs; Close never runs). Sleep panic on AfterFunc is an unrecovered goroutine panic (process crash).
Entry points already reproduced: (2a) panic in create; (2b) create == nil; (2c) panic in Sleep; (2d) panic in Wake.
Root: table maps the key slotBusy with ready open before create/Sleep/Wake; no recover to close(ready)/unmap/Close.

Agreed how (implement this):
The table owns the slotBusy/ready protocol. A panic (or nil create) in code it invoked must not leave that protocol half-finished, and must not crash the process.
- put: nil create, or panic in create, is the same as create returning an error: set createErr, slotGone, unmap, close(ready), return that error. No Close (nothing stored). Wrap panic as fmt.Errorf("reclaim: create %q: panic: %v", key, recovered).
- drop (Sleep): recover. Do not park the value asleep for reclaim. End this incarnation: Close, unmap, slotGone, close(ready). Waiters create a new incarnation.
- reclaimLocked (Wake): recover. Same end: Close, unmap, slotGone, close(ready). This Open returns an error (wrapped panic), not the pointer. A panicking hook is a broken hook, not a resume failure.
- dispose/Close panic: recover so AfterFunc cannot kill the process. close(ready) must already have happened before Close.
Do not recover inside runSleep/runWake/runClose as a silent swallow that continues as if the hook succeeded. Do not re-panic after unsticking. Reset uses the same recover.

Tests first, then fix
1. Land product tests that FAIL on current master (hang / leftover mapped key / Close never ran / process panic on AfterFunc Sleep). Then implement. Then they PASS. Existing reclaim tests stay green.
2. Do not fix the other two reclaim bugs (canceled-ctx Open returns closed value; unmap-before-Close overlap).

Example 2a (must fail before fix: later Open hangs after create panic):
recover from first Open whose create panics; assert mapped leftover; second Open with timeout must not return until the fix; after fix second Open must return and be free to create.
Example 2b: Open(..., create: nil) panics/nil deref; after recover key still busy; later Open hangs. After fix: Open returns an error, later Open with valid create works.
Example 2c: Sleep panics. Production AfterFunc path must not crash the process. Later Open must not hang. Close of the broken incarnation should run per agreed how.
Example 2d: after Sleep, second Open Wake panics; recover; third Open must not hang. After fix: Open returns wrapped panic error; later Open can create; Close ran.
