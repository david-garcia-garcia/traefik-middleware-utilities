# Tasks

## 1. Memory regression

- [ ] 1.1 Add `tokenbucket/traefik_issue13704_test.go` with `TestTraefik13704_MemoryReclaimsExpiredBuckets`: `NewMemory(15, 10, 0, 2s)`, `SetNowForTest` at a frozen Unix instant, Allow ten synthetic keys as `(status, err)` (fail if `err != nil`), jump `ttl+9s`, Allow `"fresh"`, then under the mutex `len(limiter.buckets) == 1`. Cite https://github.com/traefik/traefik/issues/13704 in the test comment. Do not copy other issues' tests or helpers. Verify `go test -short -count=1 ./tokenbucket/ -run TestTraefik13704_MemoryReclaimsExpiredBuckets` passes.

## 2. Live PTTL regression

- [ ] 2.1 Add `tokenbucket/traefik_issue13704_e2e_test.go` with `TestTraefik13704_RedisBucketExpires`: `runForEachLiveEngine` / `waitLiveClient` (`TOKENBUCKET_LIVE_REDIS`, `TOKENBUCKET_LIVE_DRAGONFLY`), helpers `liveKey13704` and `evalKey13704` in this file (`t.Helper`, Eval `pttl` on `KEYS[1]` via `simpleredis.ScriptSHA1Hex`). After three Allows with `ttl` 7s, assert `PTTL` in `(0, ttl]` on Redis and Dragonfly. Same assertion both engines. Skip under `-short` or both env vars unset. Verify `go test -short -count=1 ./tokenbucket/ -run TestTraefik13704_RedisBucketExpires` skips, then with `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` the test passes on both engines.

## 3. Integration

- [ ] 3.1 Run `go vet ./...` and `go test -short -count=1 ./...`; both green. Existing `TestMemory_FullStore*` and `TestLive_Limiter` still pass. No product edits in `memory.go`, `lua.go`, or `redis.go`.
- [ ] 3.2 Confirm the delta `openspec/changes/tokenbucket-expired-bucket-reclaim/specs/std_go_tokenbucket_allow/spec.md` still matches the landed tests (ten idle plus one new = one stored source; no `PTTL` SHALL). Run `openspec validate --change tokenbucket-expired-bucket-reclaim --strict`.

## Workflow follow-up

- Archive the change after code review; the delta folds into `std_go_tokenbucket_allow`.
