## 1. Failing tests

- [x] 1.1 Add `TestTable_CreatePanicUnsticksKey` (2a): first Open create panics (recover); after the recover the second Open with a valid create must return (on master it hangs). Assert wrapped panic error after the fix.
- [x] 1.2 Add `TestTable_NilCreateUnsticksKey` (2b): Open with nil create; after recover, Open with a valid create must return (on master it hangs). Assert `reclaim: create %q: nil create`.
- [x] 1.3 Add `TestTable_SleepPanicAfterFuncDoesNotCrash` (2c): subprocess AfterFunc Sleep panic; parent asserts child exit (non-zero on master, zero after fix). Later Open in the child must not hang; Close of the broken incarnation must run.
- [x] 1.4 Add `TestTable_WakePanicReturnsErrorAndUnsticks` (2d): after Sleep, Open Wake panics; recover; third Open must return (on master it hangs). After the fix: wrapped wake panic error, later Open creates, Close ran.
- [x] 1.5 Run `go test -count=1 -timeout 60s ./reclaim/` and confirm 1.1–1.4 fail on current table.go; existing reclaim tests still pass. Commit the tests.

## 2. Recover

- [ ] 2.1 In `put`: recover nil create and panic in create; same as create error (`createErr`, `slotGone`, unmap, `close(ready)`). No Close.
- [ ] 2.2 In `drop`: recover Sleep panic; Close, unmap, `slotGone`, `close(ready)`; do not set `createErr`; skip `reclaim_orphan`.
- [ ] 2.3 Change `reclaimLocked` to `(any, error)`; recover Wake panic; Close, unmap, `slotGone`, `close(ready)`, set `createErr`; return wrapped error.
- [ ] 2.4 In `dispose`: recover Close panic. In `Reset`: same recover around Sleep and Close.
- [ ] 2.5 Do not recover inside `runSleep` / `runWake` / `runClose`. Do not re-panic after unstick.
- [ ] 2.6 Run `go test -count=1 -timeout 60s ./reclaim/`: 1.1–1.4 pass; existing reclaim tests stay green. Commit the recover.

## 3. Usage packet

- [ ] 3.1 Update `knowledge/devdocs/std_go_reclaim.md` Wake/Sleep language: panicking Wake returns an error and ends the incarnation; panicking Sleep does not park asleep.
