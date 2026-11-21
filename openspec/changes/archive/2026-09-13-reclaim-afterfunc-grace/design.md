## Context

DestBranch `waitGraceOrWake` is `go` plus `select` on `timer.C` and `woken`. Traefik and `TestYaegi_*` interpret that code. Explore confirmed the hang on Go 1.21.13. Specs: `std_go_reclaim_value-lifecycle`. Explore: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Grace expire runs when nobody reclaims (Close / dispose / endMappedClose still happen).
- A reclaim still cancels expiry.
- Grace wait is not on the drop caller (canceled bind must not stall `Open` for grace).
- Short-watchdog interpreted regression so a missed expire fails in seconds.

**Non-Goals:**
- Converting `go t.watch`.
- Changing `expire` guards or public `Config`.
- Stacking on #77 / #78.

## Decisions

1. **`time.AfterFunc(grace, expire)` stored on the slot.** Yaegi v0.16.1 maps `time.AfterFunc` to the compiled stdlib timer. Alternative: keep `go`+`select` — rejected; dump confirmed `_select` park and missed Close.

2. **Replace `woken chan struct{}` with `graceTimer *time.Timer`.** The two cancel sites (`reclaimLocked`, `Reset`) already own the "do not expire" signal. `Stop()` is that signal. If `Stop` is false, `expire`'s existing guards skip a woken or held slot.

3. **Arm the timer after Sleep, when the slot is `slotAsleep` and still mapped.** Zero grace still expires on the drop stack. AfterFunc keeps positive grace off that stack.

4. **Permanent test is interpreted concurrent expire with a 3s watchdog.** Sequential one-shot `TestYaegi_OpenHooksRunSleepWakeClose` did not hang (`-count=5`). Isolated WaitGroup probe did not hang (5000×). Concurrent 16-key expire missed Close 3/3 on dest.

## Risks / Trade-offs

- [AfterFunc vs Stop race] → Mitigation: `expire` already returns unless `slotAsleep` and `holders == 0`; reclaim sets `slotBusy` and increments holders before Wake.
- [In-flight expire vs Reset] → Mitigation: dest already left mid-transition slots to the owner goroutine; `Stop` plus that rule stays.
- [Conflict with #77/#78] → Mitigation: report; apply on `master` only.

## Migration Plan

No signature change. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
