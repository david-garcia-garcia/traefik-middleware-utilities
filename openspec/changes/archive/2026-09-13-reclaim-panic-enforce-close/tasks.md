## 1. Tests that fail on dest

- [x] 1.1 Copy `reclaim/repro_enforce_panic_close_test.go` from the caller tree; replace only `NewTable(graceNoRace)` with `New(Config{Grace: graceNoRace})` at both call sites
- [x] 1.2 Run `go test ./reclaim -run TestRepro_ -count=1 -v` and confirm both tests FAIL with `create of incarnation 2 ran while Close of 1 was blocked` (do not weaken them)

## 2. Shared panic ending helper

- [x] 2.1 Add `endBusyAfterPanic`: flag unset is `endBusySlot` then `dispose`; flag set records `createErr` on the ended incarnation, occupies the key with a closer `slotBusy`, disposes, then `unmapAfterClose` on the closer
- [x] 2.2 Route `drop` Sleep-panic through that helper with `createErr` nil (grace-independent)
- [x] 2.3 Route `reclaimLocked` Wake-panic through that helper with the wrapped Wake error; still return that error and not the stored pointer
- [x] 2.4 Keep the closer's `ready` open across Close; do not run Close under `t.mu`; keep `runHook` recovery for Close

## 3. Confirm and usage

- [x] 3.1 Run `go test ./reclaim -run TestRepro_ -count=1` and confirm both PASS; `go test ./reclaim -count=1` stays green
- [x] 3.2 Run `go test ./... -short -count=1` (ignore pre-existing `apm_modules/.../evals/files [setup failed]`)
- [x] 3.3 Update `knowledge/devdocs/std_go_reclaim.md` Sleep/Wake panic sentences so they name stored `EnforceCloseBeforeOpen` for unmap-versus-Close order
