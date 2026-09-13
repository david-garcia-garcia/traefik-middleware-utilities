## 1. Config and constants

- [x] 1.1 Add `Logger *slog.Logger` on `Config` with a comment that nil is silent and never `Pass`. Copy it in `New`. Do not error. Do not install a discard handler
- [x] 1.2 Add `simpleredis/log.go` with exported `Msg*` constants (`simpleredis_` strings, including `MsgSocketPoisoned` and `MsgAuthLeftover`) and Error/Warn nil-check helpers. Debug stays at the call site with `Enabled`

## 2. Decision-site emits

- [x] 2.1 `New`: Debug `MsgOpen` (frozen knobs, never Pass). `Close`: Debug `MsgClose` with `idle_closed`
- [x] 2.2 `runOnConn`: keep release defer first; add recover/log `MsgPanic`/re-panic defer. Log `MsgCanceled` when stop is `Canceled`
- [x] 2.3 `exec`: Debug `MsgRetry` when `attempt > 0`. `MsgTimeout` when `libraryTimeout` maps to `errTimeout`
- [x] 2.4 `borrow`: `MsgNotFromNew`, `MsgPoolExhausted`, `MsgIdleSwept`, `MsgDial` (`idle_miss`/`stale`), `MsgCanceled` on wait cancel
- [x] 2.5 `freeInUseTurn`: Error `MsgOverFree`. Idle-cap branch in `parkIdleConn`: Debug `MsgSocketClosed` reason `idle_cap`. Do not log from the generic `!reusable` arm
- [x] 2.6 `dial`: `MsgNoAuth` on AUTH-class (logged in `do`); `MsgHandshakeFailed` on other AUTH/SELECT errors
- [x] 2.7 `do`/`ioError`/`readBulk`/`readReply`: `MsgNoAuth` on later NOAUTH; `MsgBadReply` on dirty protocol; `MsgShortBulk` on short `ReadFull` (capture `n`); `MsgTimeout` on OS/library I/O deadline; `MsgCanceled` on cancel
- [x] 2.8 `Eval`: Debug `MsgNoScript` on NOSCRIPT fallback. `storeGroupWrite`: Debug `MsgCapability` `native`/`lua`

## 3. Tests

- [x] 3.1 Capturing `slog.Handler` in `simpleredis/*_test.go`. Assert each dest-detectable event's level and attributes
- [x] 3.2 Nil-logger: every public verb works and does not panic
- [x] 3.3 Secrets: distinctive password and key; capture all lines at Debug; neither string appears
- [x] 3.4 Panic: after recovered panic, turn returned, socket closed, `OverFrees()==0`, panic still reaches the caller, `MsgPanic` emitted
- [x] 3.5 Cost: `AllocsPerRun` for Debug sites with nil logger and Debug disabled is 0. `interpretedcost_test.go` did not regress
- [x] 3.6 Yaegi: GOPATH probe with a capturing logger observes `simpleredis_open`

## 4. Usage and validate

- [x] 4.1 Update `knowledge/devdocs/std_go_simpleredis.md` (Logger optional, event names, never Pass/keys, nil silent)
- [x] 4.2 Run `openspec validate simpleredis-structured-logging --type change --strict`
- [x] 4.3 Run `go test -short ./simpleredis/...` and `go vet ./simpleredis/...`
