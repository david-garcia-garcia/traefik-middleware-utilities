# Test coverage

Ticket job (requirement): add optional `Config.Logger` structured `simpleredis_*` slog events at decision sites, with capturing tests per dest-detectable event, nil silence, secrets guard, panic log+re-panic, and Debug alloc guards.

1. [hard] Edge case untested — `simpleredis/pool.go:101-102` — `borrow` wait cancel emits `MsgCanceled` when `ctx.Done()` wins the inner select (tasks 2.4); `(none)`
   → Hold the only pool turn, start a second `Get` with a cancelable context, cancel while waiting, assert `MsgCanceled`
   Status: done
   Argument: TestLogCanceledWaitForTurn.

2. [hard] Assertion does not prove the job — `simpleredis/commands_exec.go:83-85` — mid-command cancel in `runOnConn` logs `MsgCanceled` then `MsgSocketClosed`; `TestLogSocketClosedCancel` only asserts `MsgSocketClosed`
   → Also `requireMsg(t, h, MsgCanceled, slog.LevelDebug)` in that test
   Status: done
   Argument: TestLogSocketClosedCancel also requires MsgCanceled.

3. [judgement] Happy path only — `simpleredis/commands_msetex.go:90-91` — `storeGroupWrite` logs `MsgCapability` with `path=native` on first successful native MSETEX; `TestLogDialRetryCapabilityNoScript` only hits `path=lua` via `setRejectMSetEX`
   → Exercise native MSETEX success and assert `path=native`, or skip if lua-only engines are the only CI target
   Status: skipped
   Argument: judgement; dest fake rejects native MSETEX the same way Redis 7 / Dragonfly do.
