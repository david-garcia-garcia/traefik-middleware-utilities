## 1. AfterFunc grace

- [x] 1.1 Replace `go waitGraceOrWake` + timer/`woken` select with `time.AfterFunc` on the slot in `reclaim/table.go`. `reclaimLocked` and `Reset` `Stop()` the timer. Add the Yaegi AfterFunc note.
- [x] 1.2 Remove `waitGraceOrWake` and the `woken` channel.

## 2. Interpreted regression

- [x] 2.1 Add `TestYaegi_GraceExpireDoesNotHang` (concurrent expire, 3s watchdog).
- [x] 2.2 On Go 1.21.13, dest scratch concurrent expire FAIL with `_select.func4`; after AfterFunc the watchdog PASSes at `-count=10`.

## 3. Spec, usage, siblings

- [x] 3.1 Delta `std_go_reclaim_value-lifecycle`: grace expire is a stdlib AfterFunc, not an interpreted `go`+`select`.
- [x] 3.2 Usage gotcha: Yaegi forbids interpreted `go`+`select` for the grace waiter.
- [x] 3.3 Leave `go t.watch` (probe passed). Note #77/#78 conflict risk.
- [x] 3.4 `go test` / `go vet` on `./reclaim`; `golangci-lint run`. Default-toolchain suite plus Go 1.21.13 watchdog `-count=10`.
