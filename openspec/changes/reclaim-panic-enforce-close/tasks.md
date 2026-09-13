## 1. Tests that fail on dest

- [ ] 1.1 Copy `reclaim/repro_enforce_panic_close_test.go` from the caller tree; replace only `NewTable(graceNoRace)` with `New(Config{Grace: graceNoRace})` at both call sites
- [ ] 1.2 Run `go test ./reclaim -run TestRepro_ -count=1 -v` and confirm both tests FAIL with `create of incarnation 2 ran while Close of 1 was blocked` (do not weaken them)

## 2. Shared panic ending helper

- [ ] 2.1 Add `endBusyAfterPanic` that records `createErr` under `t.mu` while still mapped, then `endMappedClose` when stored `EnforceCloseBeforeOpen` is set, else `endBusySlot` then `dispose`
- [ ] 2.2 Route `drop` Sleep-panic through that helper with `createErr` nil (grace-independent)
- [ ] 2.3 Route `reclaimLocked` Wake-panic through that helper with the wrapped Wake error; still return that error and not the stored pointer
- [ ] 2.4 Do not close `ready` before Close on the enforced path; do not run Close under `t.mu`; keep `runHook` recovery for Close

## 3. Confirm and usage

- [ ] 3.1 Run `go test ./reclaim -run TestRepro_ -count=1` and confirm both PASS; `go test ./reclaim -count=1` stays green
- [ ] 3.2 Run `go test ./... -short -count=1` (ignore pre-existing `apm_modules/.../evals/files [setup failed]`)
- [ ] 3.3 Update `knowledge/devdocs/std_go_reclaim.md` Sleep/Wake panic sentences so they name stored `EnforceCloseBeforeOpen` for unmap-versus-Close order
