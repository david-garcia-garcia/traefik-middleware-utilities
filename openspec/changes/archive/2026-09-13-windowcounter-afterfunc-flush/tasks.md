## 1. AfterFunc flush

- [x] 1.1 Replace `go flushLoop` + ticker/stop select with `time.AfterFunc` + `flushBusy` in `windowcounter/limiter.go`. Keep `stopping`. Add the Yaegi AfterFunc note.
- [x] 1.2 Point `TestClose_StopsTickerAndKeepsRedis` and `TestWake_StartsTickerAfterSleep` at `flushTimer` instead of `stop`

## 2. Interpreted regression

- [x] 2.1 Add `windowcounter/yaegi_flush_stop_test.go` (`TestYaegi_BufferedShareSleepDoesNotHang` 3s watchdog; `TestYaegi_MethodFlushStopDoesNotHang`)
- [x] 2.2 On Go 1.21.13, show the watchdog FAIL on dest (scratch dump) and PASS after AfterFunc

## 3. Spec, usage, siblings

- [x] 3.1 Delta `std_go_windowcounter_sync-flush`: flush timer not interpreted goroutine; Sleep/Wake/Close unchanged
- [x] 3.2 Usage gotcha: Yaegi forbids interpreted `go`+`select` for flush
- [x] 3.3 Note reclaim `waitGraceOrWake` as debt; do not convert it here
- [x] 3.4 `go test -short` / `go vet` on `./windowcounter`; `openspec validate windowcounter-afterfunc-flush --strict`. Local `-race` needs gcc (not present); CI Unit race job measures it.
