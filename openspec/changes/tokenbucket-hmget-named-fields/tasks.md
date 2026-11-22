# Tasks

## 1. Named field read

- [ ] 1.1 Add `tokenbucket/traefik_issue14297_e2e_test.go` with `TestTraefik14297_HGetAllFieldOrder` and helpers suffixed `14297` (`control_last_first`, `tokens_first_listpack`, `tokens_first_hashtable`; hashtable may skip). Gate it with `runForEachLiveEngine` / `waitLiveClient`. Verify the file compiles with `go test -short -count=1 -run TestTraefik14297_HGetAllFieldOrder ./tokenbucket/`.
- [ ] 1.2 In `tokenbucket/lua.go`, replace `HGETALL` and `#rl_source == 4` with `HMGET` of `last` and `tokens`, seed when either value is not truthy, and update the header to cite traefik/traefik#14297 and PR #14300. Leave the rest of the script, `allowScriptDigest`, and `tokenbucket/memory.go` unchanged. Verify `allowScript` contains `hmget` and does not contain `hgetall`.
- [ ] 1.3 Run `TestTraefik14297_HGetAllFieldOrder` on live Redis and Dragonfly (`TOKENBUCKET_LIVE_REDIS`, `TOKENBUCKET_LIVE_DRAGONFLY`) and verify `control_last_first` and `tokens_first_listpack` pass on both engines.

## 2. Existing live paths

- [ ] 2.1 Run `go vet ./...`, `go test -short -count=1 ./...`, and the tokenbucket live tests (`TestLive_Limiter` and `TestYaegiLive_RedisAndDragonfly`) with the live env vars. Verify all of those commands pass.

## Workflow follow-up

- Archive the change after review.
- Publish the PR summary with the upstream issue and PR links.
