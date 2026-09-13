# Explore
IssueKey: 2026-09-13-windowcounter-bug-yaegi-flush-hang

## Concepts

Buffered `windowcounter` starts a flush worker at `New` when `sync_rate > 0`. Dest uses `go l.flushLoop` plus an interpreted `select` on `ticker.C` and `stop`. `Sleep`/`Close` close `stop` then `wg.Wait()`. Yaegi v0.16.1 implements `select` as `reflect.Select` (`interp._select` at `run.go:3815`) and compiled calls as `callf(in)` (`run.go:1330`). `sync.WaitGroup.Wait` is a compiled stdlib call, so a missed `close(stop)` looks like Eval stuck at `run.go:1330`.

CI is Go 1.21.13. Local default is Go 1.25.6. The hang reproduced on `GOTOOLCHAIN=go1.21.13` and did not show up in 30 iterations on 1.25.6.

## Decisions

- Supercede PR 61 for this deadlock. Dest already has `stopping`. PR 61's remaining delta is compiled Sleep/Wake race tests/spec; this run does not merge that branch. AfterFunc still keeps the stopping intent so Wake cannot start a timer while Sleep/Close is stopping.
- Do not touch PR 69.
- Fix with self-re-arming `time.AfterFunc` (Yaegi v0.16.1 maps `time.AfterFunc` in `stdlib/go1_21_time.go`). Not opportunistic Take/Peek flush: that would leave idle deltas unflushed.
- Isolated ticker+stop method probe (200 cycles × 5 Evals) did **not** hang. The hang needs the interpreted `windowcounter` limiter (method `go l.flushLoop`, `Sleep` → `stopFlushAndWait`). Permanent repro is interpreted `BufferedShare` / Sleep with a short watchdog, run under Go 1.21.
- Sibling sweep: fix only `windowcounter`. Note `reclaim/table.go` `waitGraceOrWake` (same `go` + timer/channel `select`).

## Open questions

- Q: Does interpreted `flushLoop` `select` miss `close(stop)` so `wg.Wait` never returns?
  Rank: bounded asked — existing flushLoop callers enumerated (`New`, `Sleep`, `Wake`, `Close`, `startFlushLocked`, `stopFlushAndWait`); requirement Desired line 1.
  Decision: resolved — CONFIRMED on Go 1.21.13. Scratch `TestScratchYaegiBufferedShareSleep` timed out at 8s. Goroutine 60 `sync.(*WaitGroup).Wait` via Yaegi `callBin`; goroutines 62 and 63 `interp._select.func4` `run.go:3815` (`created by interp.call.func9` `run.go:1322`, the `go callf(in)` method path). Fake Redis accept/read goroutines were idle, not the blocker. Same stack shape as CI run 34766385613 at `evalTakeprobe` `limiter_yaegi_test.go:51`. Did not hang on Go 1.25.6 (30×). Isolated closure/method probe without `windowcounter` sources did not hang (20 + 1000 start/stop cycles).
  By: explore

- Q: `time.AfterFunc` or opportunistic Take/Peek flush?
  Rank: bounded asked — requirement Desired line 2 names both and forbids silent opportunistic; existing Sleep/Wake/Close callers stay, timer owner changes.
  Decision: resolved — AfterFunc. Periodic flush preserved. Opportunistic idle skip is a behavior change.
  By: explore

- Q: Build on, coordinate with, or supercede `2026-09-13-windowcounter-bug-sleep-wake-hang` (PR 61)?
  Rank: additive asked — requirement Desired line 6.
  Decision: resolved — supercede for this deadlock. Dest already has `stopping`. PR 61 does not remove interpreted `go`+`select`.
  By: explore

- Q: Which sibling `go`+channel-`select` hits are in scope to fix?
  Rank: additive asked — requirement Desired line 5 (cheap identical only).
  Decision: resolved — fix `windowcounter/limiter.go:364` / `:401` only. Hits: `reclaim/table.go:419` `go t.waitGraceOrWake` + `:427` select (timer and `woken`) — not cheap, note. `reclaim/table.go:345` `go t.watch` is `<-ctx.Done()` poll not select. `simpleredis/pool.go:60,85,90` and `commands_exec.go:130` select on the caller goroutine, not a spawned flush worker. `tokenbucket` and `backendbackoff` production code have no `go`+`select`.
  By: explore
