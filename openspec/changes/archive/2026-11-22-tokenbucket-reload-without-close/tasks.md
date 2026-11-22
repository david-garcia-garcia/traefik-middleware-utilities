# Tasks

## 1. Live regression test

- [x] 1.1 Add `tokenbucket/traefik_issue13618_e2e_test.go` with `testCountingProxy13618` and `startTestCountingProxy13618` (open counter, bidirectional copy, no write hold) and `liveKey13618`. Verify `go test -short -count=1 ./tokenbucket/` still skips live and compiles the new file
- [x] 1.2 Add `TestTraefik13618_ReloadWithoutClose` using dest `runForEachLiveEngine` / `waitLiveClient`, 50 reloads of `simpleredis.New` + `NewRedis` + `status, err := limiter.Allow(...)`, drop without Close, GC settle, `proxy.open == 0`, `leakedGoroutines > 2` fails. Verify Allow is two return values and helpers are suffixed `13618`

## 2. Local gate

- [x] 2.1 Run `go vet ./...` and `go test -short -count=1 ./...` and verify both pass
- [x] 2.2 Run the live test of `./tokenbucket/` with `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` (and the existing SIMPLEREDIS live vars if that package is in the same command) and verify Redis and Dragonfly subtests pass with `proxy.open == 0` after GC
