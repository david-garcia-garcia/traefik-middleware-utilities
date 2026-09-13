## 1. Tests first (must FAIL on dest)

- [x] 1.1 Add `tokenbucket/repro_hunt_clock_last_backward_test.go` with `TestRepro_StaleNowRewindsLastDoubleRefill` subtests `sequential` and `goroutines_stale_samples_before_fresh_lock`, dest `Allow(ctx, key)`, helpers copied or inlined
- [x] 1.2 Assert `allowScript` HSET last is persist-max, not raw `t`
- [x] 1.3 Run `go test -short ./tokenbucket -run TestRepro_StaleNowRewindsLastDoubleRefill -count=1` and record FAIL (do not weaken tests)

## 2. Persist-max and lock-order clock

- [x] 2.1 `consumeOne`: capture previous last before the elapsed clamp; return `max(previous last, nowMicro)`; keep the elapsed clamp
- [x] 2.2 Lua: HSET last is `max(bucket.last, t)`; keep the elapsed clamp and the MIT notice
- [x] 2.3 Memory `Allow`: read `now` after `mu.Lock()`; `expireAt` uses that now

## 3. Proofs

- [x] 3.1 Re-run `go test -short ./tokenbucket -run TestRepro_StaleNowRewindsLastDoubleRefill -count=1` until PASS
- [x] 3.2 Run `go test -short ./tokenbucket/...` until passing

## 4. Validate

- [x] 4.1 Run `openspec validate --change tokenbucket-last-not-rewind --strict`
