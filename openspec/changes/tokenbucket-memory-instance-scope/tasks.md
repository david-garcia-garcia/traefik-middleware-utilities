# Tasks

## 1. Compiled Memory scope tests

- [x] 1.1 Add `tokenbucket/traefik_issue13706_test.go` with helper `countAdmits13706` (reads `Status.Allowed`, issue suffix only, no shared helper file) and a parent test with subtests `sharedMemory` (two callers, one `*Memory`, same key, `maxDelay` 0, total admits `burst`) and `separateMemory` (two `NewMemory`, same key, total admits `2*burst`, comment cites `traefik/traefik#13706`). Verify `go test -short -count=1 -run TestTraefik13706 ./tokenbucket/` passes.
- [x] 1.2 Leave `TestRedis_TwoInstancesShareBurst` unchanged and verify `go test -short -count=1 -run TestRedis_TwoInstancesShareBurst ./tokenbucket/` still passes.

## 2. Live Redis scope test

- [x] 2.1 Add `tokenbucket/traefik_issue13706_e2e_test.go`: two `*Redis` on one `liveKey13706` (run nonce + `t.Name()` + suffix, not bare `t.Name()`), `maxDelay` 0, total admits `burst`. Gate with existing `runForEachLiveEngine` / `waitLiveClient` and `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`. Do not extract those helpers. Verify `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 go test -count=1 -run TestTraefik13706 ./tokenbucket/` passes (skips under `-short`).
- [x] 2.2 Leave `TestLive_Limiter` / `twoInstancesShare` unchanged and verify the same live env `go test -count=1 -run TestLive_Limiter ./tokenbucket/` still passes.

## 3. GoDoc and usage

- [x] 3.1 Extend GoDoc on `Memory` and `NewMemory` in `tokenbucket/memory.go`: each value is its own key space; to share a budget across routers or reloads, share one `*Memory` (example: `reclaim` keyed by middleware name). Do not import `reclaim`. Verify `go doc github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket.Memory` and `NewMemory` show that scope.
- [x] 3.2 Extend GoDoc on `Redis` and `NewRedis` in `tokenbucket/redis.go`: state lives on the server; any limiter on the same key shares it; the key prefix defines the scope. Verify `go doc` for `Redis` and `NewRedis` show that scope.
- [x] 3.3 Add the Memory-instance isolation gotcha to `knowledge/devdocs/std_go_tokenbucket.md` next to the existing Redis-share line (two `*Memory` admit `2*burst`; share one value for one budget). Do not change Language or README. Verify the gotcha names both stores.

## 4. Local gate

- [x] 4.1 Run `go vet ./...` and `go test -short -count=1 ./...` and confirm both exit 0.
- [x] 4.2 Run the live tests of `tokenbucket` with `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` (and the SIMPLEREDIS auth addrs if that package is pulled in) and confirm they pass.
- [x] 4.3 Run `openspec validate --change tokenbucket-memory-instance-scope --strict` and confirm it passes. Do not edit `std_go_tokenbucket_lua-eval` or `std_go_tokenbucket_status`.
