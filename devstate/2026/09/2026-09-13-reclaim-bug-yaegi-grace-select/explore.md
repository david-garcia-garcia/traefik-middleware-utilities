# Explore
IssueKey: 2026-09-13-reclaim-bug-yaegi-grace-select

## Concepts

`drop` of the last holder with positive grace starts `go t.waitGraceOrWake`, which parks in a two-case `select` on `time.Timer.C` and `incarnation.woken`. Yaegi v0.16.1 implements `select` as `reflect.Select` (`interp._select` at `run.go:3815`) and `go method(...)` as `go callf(in)` (`run.go:1322`). A missed timer wake means `expire` never runs.

`Open` of a still-mapped `slotAsleep` does not wait for that waiter: `reclaimLocked` closes `woken` and Wakes on this stack. A later `Open` therefore does not wedge. The stranded waiter leaks the stored value and skips Close.

`e2e/reclaimprobe/plugin.go` holds `reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})` at package scope, so every Traefik reload drop arms this waiter.

CI is Go 1.21.13. Local default is Go 1.25.6. Reproduction requires `GOTOOLCHAIN=go1.21.13`.

## Decisions

- Reproduce first on Go 1.21.13. Confirmed: interpreted concurrent expire (16 keys × rounds, 1ms grace) misses Close and leaves `interp._select.func4` parked. 3/3 `-count=3`. Isolated ticker-shaped probe (5000 start/stop + WaitGroup) did not hang — same as windowcounter's isolated probe.
- Fix with `time.AfterFunc(grace, expire)` stored on the slot; wake/Reset `Stop()` the timer. Do not keep `go` + `select`. Drop caller stays off the grace wait. `expire` guards stay.
- Replace `woken chan struct{}` with the timer pointer. Same two call sites (`reclaimLocked`, `Reset`) cancel expiry.
- Permanent regression is interpreted concurrent expire with a 3s watchdog (`TestYaegi_GraceExpireDoesNotHang`), mirroring `TestYaegi_BufferedShareSleepDoesNotHang`.
- `go t.watch` stays. `waitCtx` is `<-done` or a 20ms `Err` poll, not a two-case select.
- Target `origin/master`. Do not stack on reclaim bug branches. Report conflict risk vs #77 and #78.

## Open questions

- Q: Does interpreted `waitGraceOrWake` `select` miss the timer so `expire` never runs?
  Rank: bounded asked — existing call sites enumerated (`drop` L419, `waitGraceOrWake` L424, `reclaimLocked` L298, `Reset` L473); requirement Desired line 1.
  Decision: resolved — CONFIRMED on Go 1.21.13. `TestScratchYaegiGraceConcurrentExpire` `-count=3` missed Close (rounds 4, 1, 2; one miss each). Dump: goroutine in `reflect.rselect` / `interp._select.func4` `run.go:3815`, `created by interp.call.func9` `run.go:1322`. `TestYaegi_OpenHooksRunSleepWakeClose` `-count=5` passed (not aggressive). Sequential expire loop 2000× passed. Isolated 5000× WaitGroup probe passed. Toolchain `go1.21.13 windows/amd64`.
  By: explore

- Q: `time.AfterFunc` or keep `go`+`select`?
  Rank: bounded asked — requirement Desired line 2 names AfterFunc and forbids speculative refactor without a hang.
  Decision: resolved — AfterFunc. Hang dump exists. Wake/Reset Stop the timer. `expire` guards unchanged.
  By: explore

- Q: If the grace timer wake is missed, does a later `Open` wedge, or only leak?
  Rank: additive asked — requirement Desired line 5.
  Decision: resolved — leak, not a wedged `Open`. Missed timer: `expire` never runs, so `dispose` and `endMappedClose` never run. Slot stays `slotAsleep` and mapped. `reclaimLocked` still Wakes. Traefik plugin path (`e2e/reclaimprobe/plugin.go`) leaks the stored value and never runs Close; a later `New`/`Open` of the same key reclaims.
  By: explore

- Q: Is `go t.watch` the same hang shape?
  Rank: additive asked — requirement Desired line 4.
  Decision: resolved — agree with the windowcounter sweep. `TestScratchYaegiWatchNilDone` 80 interpreted nil-Done holders on Go 1.21.13 all Closed (2.585s). `waitCtx` is `<-done` or a 20ms `Err` poll, not a two-case select. Do not convert `watch` in this change.
  By: explore

- Q: Coordinate with in-flight `table.go` PRs #77 and #78?
  Rank: additive asked — requirement Desired / Affected.
  Decision: resolved — proceed on `master`. Both still have `waitGraceOrWake`. #78 (+33/−7) and #77 (+54/−13) will conflict around `drop` / `watch`. Other local reclaim branches also rewrite `table.go` (`canceled-open-close`, `hook-panic-busy`, `unmap-before-close`, `owned-table`). Do not narrow those diffs.
  By: explore
