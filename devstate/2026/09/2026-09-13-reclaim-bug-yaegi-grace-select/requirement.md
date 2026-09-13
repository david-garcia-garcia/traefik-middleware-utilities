# Requirement
IssueKey: 2026-09-13-reclaim-bug-yaegi-grace-select

## Problem
Yaegi v0.16.1 can miss a channel wake when an interpreted goroutine parks in a two-case `select` on a timer and a stop/wake channel. PR #75 fixed that shape in `windowcounter`. `reclaim/table.go` `waitGraceOrWake` is the same shape and is armed under Yaegi in CI (`TestYaegi_OpenHooksRunSleepWakeClose`) and in the Traefik plugin probe (`e2e/reclaimprobe/plugin.go`). A missed grace-timer wake can leave a sleeping incarnation mapped forever.

## Current (code)
- `reclaim/table.go` `drop` last holder: Sleep, orphan, then `go t.waitGraceOrWake` when grace > 0 (around L419).
- `reclaim/table.go` `waitGraceOrWake` (L424–431): `time.NewTimer(grace)` then `select` on `wait.C` vs `woken`.
- `reclaim/table.go` `expire` (L437): ends a still-asleep incarnation; either `endMappedClose` (`EnforceCloseBeforeOpen`) or unmap + `dispose`.
- `reclaim/table.go` `reclaimLocked` (L298–301): `close(incarnation.woken)` to cancel the grace wait, then Wake.
- `reclaim/table.go` `dropWhenDone` (L345): `go t.watch` only when `ctx.Done()` is nil; otherwise `context.AfterFunc`. `watch` calls `waitCtx`, which is `<-done` or a 20ms `Err` poll — not a two-case select.
- `reclaim/yaegi_test.go` `TestYaegi_OpenHooksRunSleepWakeClose` (~L154): interpreted `New(Config{Grace: 20 * time.Millisecond})`, one sleep/wake/close.
- `e2e/reclaimprobe/plugin.go` L34: package-level `reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})` (10s).
- `windowcounter/limiter.go` on PR #75: `time.AfterFunc` replaced `go` + `select` on ticker/stop.
- `knowledge/debt/2026-09-13-reclaim-grace-select.md` lives on the windowcounter branch, not on `origin/master`.

## Desired
- Reproduce the hang on Go 1.21.13 with a goroutine dump naming the interpreted select frame before changing product code. Force `GOTOOLCHAIN=go1.21.13`. A green run on 1.25.6 is not evidence.
- Remove the spawned-goroutine-parked-in-select shape from the grace waiter, most likely via `time.AfterFunc` armed for `grace`, with the wake path stopping the timer. Keep the contract: grace wait not on the drop caller; wake cancels expiry; `expire` guards (`slotAsleep`, `holders > 0`, `EnforceCloseBeforeOpen` → `slotBusy`) untouched; no timer or goroutine leak.
- Permanent Yaegi watchdog regression test, mirroring `TestYaegi_BufferedShareSleepDoesNotHang`.
- Verify `go t.watch` on 1.21.13 and report agreement or disagreement with the windowcounter sweep.
- Report the Traefik-plugin consequence if the grace timer wake is missed (which of `expire`, `dispose`, `endMappedClose` never run; whether a later `Open` wedges).
- PR against `master`. Green CI with run id. Delivery card.

## Affected
- `reclaim/table.go` (`waitGraceOrWake` and its call site in `drop`)
- `reclaim` Yaegi tests (new watchdog; existing `TestYaegi_OpenHooksRunSleepWakeClose`)
- Possibly `openspec/specs/std_go_reclaim_value-lifecycle` / `std_go_reclaim_context-lease` if the delta names the Yaegi-safe waiter
- In-flight `table.go` PRs: #78 (`2026-09-13-reclaim-bug-panic-ignores-enforce`), #77 (`2026-09-13-reclaim-bug-nildone-watcher-leak`); other reclaim branches also rewrite `table.go`

## Out of scope
- Rewriting `expire` / `dispose` / `endMappedClose` guards
- Changing grace duration or public `Config`
- Replacing `go t.watch` unless 1.21.13 evidence shows it hangs
- Windowcounter or simpleredis further Yaegi work
- Merging or rebasing onto reclaim feature branches

## Unknowns
- Whether the existing one-shot Yaegi hook test hangs on 1.21.13, or a scratch loop of drop/wake is required
- Whether a missed timer wake only leaks the incarnation or can also park a later `Open` (read of `Open`/`reclaimLocked` says later Open can still reclaim while the waiter is stranded; confirm against a hang if one is obtained)
- Merge conflict risk vs #77/#78 once those land

## Tensions
- Ticket says do not fix without reproduction; if a determined 1.21.13 campaign cannot hang, stop and report — do not speculative-refactor.
- Ticket asks to keep `expire` guards untouched while replacing the waiter that calls `expire`.
- Several OPEN reclaim PRs also edit `table.go`; this change must target `master` and report conflict risk rather than narrow those diffs.
