# Tasks

## 1. Effective TTL

- [x] 1.1 After `validateClock` succeeds, store `max(callerTTL, refillFloor)` on `clockConfig.ttl` from both `NewMemory` and `NewRedis`. `refillFloor` is `ceil(burst/rate + maxDelay.Seconds())` whole seconds, clamped to `(math.MaxInt64 / int64(time.Second)) * time.Second`. Verify `go test -short -count=1 ./tokenbucket/ -run 'TestTTLFloor|TestTTLTruncation'` passes: short ttl raised, long ttl kept, maxDelay included, overflow clamped, fractional 1500ms still `errTTL`, and the rate-1 burst-2 maxDelay-1µs ARGV ttl is `"3"`.

## 2. Upstream repros

- [x] 2.1 Add `tokenbucket/traefik_issue13957_test.go` with helpers suffixed `13957`. `TestTraefik13957_MemoryIdleBucketRecreatedFull`: rate 0.5, burst 30, maxDelay 0. Control ttl 60s and Traefik ttl 3s both admit 2 after a 5s pause, not 30. Verify `go test -short -count=1 ./tokenbucket/ -run TestTraefik13957_MemoryIdleBucketRecreatedFull` passes.
- [x] 2.2 Add `tokenbucket/traefik_issue13957_e2e_test.go` using `runForEachLiveEngine` and `waitLiveClient`. `TestTraefik13957_RedisIdleBucketRecreatedFull` skips under `-short` or without the live env vars. With a 4s pause the traefik_ttl and control cases admit at most 3. Verify the test skips under `go test -short -count=1 ./tokenbucket/ -run TestTraefik13957_RedisIdleBucketRecreatedFull`.

## 3. Docs in this change

- [x] 3.1 GoDoc on `NewMemory` and `NewRedis`: ttl is a minimum, the library raises it to the refill time, and cite traefik/traefik#13957. Update `knowledge/devdocs/std_go_tokenbucket.md` so the ttl sentence matches. Verify both files contain `13957` and describe the raise, and `go test -short -count=1 ./tokenbucket/` passes.

## 4. Local gate

- [x] 4.1 Run `go vet ./...` and `go test -short -count=1 ./...` from the worktree. Then run the tokenbucket live tests with `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379` and `TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` (not `-short`). Verify vet and both test commands exit 0.

## Workflow follow-up

- Archive this change after code review and the devdocs impact check.
- The delivery card names https://github.com/traefik/traefik/issues/13957. There is no upstream pull request.
