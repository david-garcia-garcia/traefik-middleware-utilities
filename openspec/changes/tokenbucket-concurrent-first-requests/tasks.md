# Tasks

## 1. Memory concurrent first requests

- [ ] 1.1 Add `tokenbucket/traefik_issue14328_test.go` with `TestTraefik14328_MemoryConcurrentFirstRequests` and in-file `concurrentFirstAdmits14328` (`t.Helper()`): 20 rounds, new `NewMemory(1.0/60, 5, 500ms, 120s)` per round, 16 goroutines on a `ready` WaitGroup then `close(start)` barrier, opaque key `"192.0.2.1"`, count `Status.Allowed` from dest `(Status, error)`. Assert admits ≤ 5; `Allow` error fails the test. No product edits. Verify `go test -short -count=1 -run '^TestTraefik14328_MemoryConcurrentFirstRequests$' ./tokenbucket/` passes in under a few seconds. If any round admits more than burst, stop (do not patch `memory.go`).

## 2. Live Redis and Dragonfly concurrent first requests

- [ ] 2.1 Add `tokenbucket/traefik_issue14328_e2e_test.go` with `TestTraefik14328_RedisConcurrentFirstRequests` gated by `runForEachLiveEngine(t, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY", ...)`. In this file: `waitLiveClient14328` (`simpleredis.New` with PoolSize 16, MaxIdleConns 16, PoolTimeout 2s, Eval `"return 1"` wait up to 15s) and `liveKey14328` (`t.Helper()`). Reuse `concurrentFirstAdmits14328` from the unit file (same package). Do not change `waitLiveClient`. 10 rounds, one `NewRedis` at the same load as 1.1, new opaque key per round. Verify `go test -short -count=1 -run '^TestTraefik14328_RedisConcurrentFirstRequests$' ./tokenbucket/` skips, `git diff -- tokenbucket/limiter_e2e_test.go tokenbucket/memory.go tokenbucket/redis.go tokenbucket/lua.go` is empty, and `go test -count=1 -run '^TestTraefik14328_RedisConcurrentFirstRequests$' ./tokenbucket/` with `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379` and `TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` passes (max admits 5 on both engines). If any round admits more than burst, stop (do not patch `redis.go` / `lua.go`).

## 3. Integration

- [ ] 3.1 `go vet ./...` and `go test -short -count=1 ./...` green; `golangci-lint` v1.63.4 clean on the new files (unused/unparam/godot/thelper). Confirm no Yaegi file, no shared helper file, and no copy of the upstream-repro worktree tests. Product tree besides the two new test files is unchanged.

## Workflow follow-up

- Archive folds the deltas into `std_go_tokenbucket_allow` and `std_go_tokenbucket_lua-eval`.
- The PR summary MUST keep https://github.com/traefik/traefik/issues/14328 and https://github.com/traefik/traefik/issues/13957 and state the upstream defect does not occur here (one mutex on Memory; atomic Eval on Redis).
