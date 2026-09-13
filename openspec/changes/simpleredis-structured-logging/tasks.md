## 1. Config and constants

- [ ] 1.1 Add `Logger *slog.Logger` on `Config` with a comment that nil is silent and never `Pass`. Copy it in `New`. Do not error. Do not install a discard handler
- [ ] 1.2 Add `simpleredis/log.go` with exported `Msg*` constants (`simpleredis_` strings, including `MsgSocketPoisoned` and `MsgAuthLeftover` undeclared-fire) and Error/Warn nil-check helpers. Debug stays at the call site with `Enabled`

## 2. Decision-site emits

- [ ] 2.1 `New`: Debug `MsgOpen` (frozen knobs, never Pass). `Close`: Debug `MsgClose` with `idle_closed`
- [ ] 2.2 `runOnConn`: keep release defer first; add recover/log `MsgPanic`/re-panic defer. Log `MsgCanceled` when stop is `Canceled`
- [ ] 2.3 `exec`: Debug `MsgRetry` when `attempt > 0`. `MsgTimeout` when `libraryTimeout` maps to `errTimeout`
- [ ] 2.4 `borrow`: `MsgNotFromNew`, `MsgPoolExhausted`, `MsgIdleSwept`, `MsgDial` (`idle_miss`/`stale`), `MsgCanceled` on wait cancel
- [ ] 2.5 `freeInUseTurn`: Error `MsgOverFree`. `release` idle-cap branch: Debug `MsgSocketClosed` reason `idle_cap`. Do not log from the generic `!reusable` arm
- [ ] 2.6 `dial`: `MsgNoAuth` on AUTH-class; `MsgHandshakeFailed` on other AUTH/SELECT errors
- [ ] 2.7 `do`/`ioError`/`readBulk`/`readReply`: `MsgNoAuth` on later NOAUTH; `MsgBadReply` on dirty protocol; `MsgShortBulk` on short `ReadFull` (capture `n`); `MsgTimeout` on OS/library I/O deadline; `MsgCanceled` on cancel
- [ ] 2.8 `Eval`: Debug `MsgNoScript` on NOSCRIPT fallback. `storeGroupWrite`: Debug `MsgCapability` `native`/`lua`

## 3. Tests

- [ ] 3.1 Capturing `slog.Handler` in `simpleredis/*_test.go`. Assert each dest-detectable event's level and attributes
- [ ] 3.2 Nil-logger: every public verb works and does not panic
- [ ] 3.3 Secrets: distinctive password and key; capture all lines at Debug; neither string appears
- [ ] 3.4 Panic: after recovered panic, turn returned, socket closed, `OverFrees()==0`, panic still reaches the caller, `MsgPanic` emitted
- [ ] 3.5 Cost: `AllocsPerRun` for Debug sites with nil logger and Debug disabled; report numbers. `interpretedcost_test.go` must not regress
- [ ] 3.6 Yaegi: GOPATH probe with a capturing logger observes `simpleredis_open` (and one command-path event if cheap)

## 4. Usage and validate

- [ ] 4.1 Update `knowledge/devdocs/std_go_simpleredis.md` (Logger optional, event names, never Pass/keys, nil silent)
- [ ] 4.2 Run `openspec validate simpleredis-structured-logging --type change --strict`
- [ ] 4.3 Run `go test -short ./simpleredis/...` and `go vet ./simpleredis/...`
