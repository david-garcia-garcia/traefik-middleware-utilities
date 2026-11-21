## 1. Failing repro first

- [x] 1.1 Add `windowcounter/repro_stale_previous_test.go`: two buffered clients, one Take each at frozen `start+9s` (10s window, limit 2), A.Sleep then B.Sleep, clock `start+10s`, A's Take must deny at estimated 3
- [x] 1.2 Run that test on dest `limiter.go` and confirm it FAILs (admits estimated 2)

## 2. Take-only previous GET

- [x] 2.1 In `takeBuffered`, GET previous when that key is in memory, `localDelta == 0`, and `expireAt > 0`; set `redisKnown`; if `localDelta > 0` keep memory; do not INCR previous; do not write current `expireAt` onto previous
- [x] 2.2 Leave Peek on `bufferedCountLocked` (no GET previous every call). Failed GET returns from Take like `windowLocked`
- [x] 2.3 Re-run the repro and confirm it PASSES. Run `go test -short -count=1 -timeout 60s ./windowcounter`

## 3. Usage

- [x] 3.1 Update `knowledge/devdocs/std_go_windowcounter.md`: buffered Take GETs previous when `localDelta == 0` and `expireAt` is set; Peek does not

## 4. Validate

- [x] 4.1 Run `openspec validate windowcounter-stale-previous --strict`
