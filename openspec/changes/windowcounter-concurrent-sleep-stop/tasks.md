# Tasks

## 1. Stop counter

- [ ] 1.1 Replace `stopping bool` with `stoppers int` and rewrite `stopFlushAndWait`, `Wake`, and `flushAfterTick` per design.md Decisions 1-2. Do not touch `flushPendingLocked`. Verify `go test -count=1 ./windowcounter/` passes existing tests, including `zzz_repro_sleep_wake_race_test.go` and the Yaegi flush-stop tests.

## 2. Tests

- [ ] 2.1 Add `windowcounter/zzz_concurrent_sleep_stop_test.go` with cases (a)-(d) of Decision 3, helpers suffixed `ConcurrentSleepStop`. Case (a) or (b) must fail on the old code. Verify `go test -count=1 -race -run ConcurrentSleepStop ./windowcounter/`.

## 3. Local gate

- [ ] 3.1 `go vet ./...` and `go test -short -count=1 ./...`; then `go test -count=1 ./windowcounter/` with the live engine env vars if available. Verify all exit 0.
