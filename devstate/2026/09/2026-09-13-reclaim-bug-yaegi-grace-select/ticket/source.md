# Reclaim grace waiter still uses interpreted go + select

Sibling of PR #75 (`2026-09-13-windowcounter-bug-yaegi-flush-hang`). Yaegi v0.16.1 `_select` can miss a wake and park forever when an interpreted goroutine sits in a two-case `select` on a timer channel and a wake channel. Confirmed on Go 1.21.13, not on Go 1.25.6. CI pins Go 1.21.

`reclaim/table.go` `waitGraceOrWake` is the same shape:

- `go t.waitGraceOrWake(key, incarnation, woken, grace)`
- `select` on `wait.C` and `woken`

Two live interpreted paths arm a non-zero grace today:

1. `reclaim/yaegi_test.go` `TestYaegi_OpenHooksRunSleepWakeClose` uses `Grace: 20 * time.Millisecond`.
2. `e2e/reclaimprobe/plugin.go` uses `reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})` so the Traefik plugin path arms grace on every drop.

Required:

- Reproduce first on Go 1.21.13 (`GOTOOLCHAIN=go1.21.13`). Do not fix without a hang dump naming the interpreted select frame.
- Fix along the windowcounter line: remove spawned-goroutine-parked-in-select, most likely `time.AfterFunc`. Keep the contract: grace wait must not run on the drop caller; wake still cancels expiry; `expire` guards stay; no timer or goroutine leak.
- Permanent Yaegi regression test with a watchdog, mirroring `TestYaegi_BufferedShareSleepDoesNotHang`.
- Verify `go t.watch` on 1.21.13 (polls `<-ctx.Done()`, not a two-case select).
- Report user-visible consequence if the grace timer wake is missed: leak vs wedge later `Open`; which of `expire`, `dispose`, `endMappedClose` never run.
- Check in-flight branches that also modify `table.go`.
- PR against `master` (not a stacked feature branch). Green CI with run id. Delivery card.
