# Dead

1. [hard] Leftover production path — `reclaim/table.go:147` — `dispose` has no callers outside tests after drop/expire moved to `runClose` plus the dispose log
   ```
   func dispose(key string, hooks Hooks, logger *slog.Logger) {
   	runClose(hooks)
   	logger.Debug(MsgDispose, "key", key)
   }
   ```
   Grep `\bdispose\(` in `*.go`: definition plus `Table.Reset` at `reclaim/table.go:412` and `:414`. Grep `.Reset(` / `ResetWith(` in `*.go`: only `*_test.go` plus tests-only `Reset`/`ResetWith` in `reclaim/default.go`. No other package calls `reclaim.Reset`.
   → Delete `dispose`; inline `runClose` and the dispose log at the Reset sites that still need Close-then-log together
   Status: done
   Argument: deleted `dispose`; Reset inlines `runClose` then the dispose log (`80328a9`).
