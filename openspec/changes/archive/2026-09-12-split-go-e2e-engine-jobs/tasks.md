## 1. CI job split

- [x] 1.1 Replace `.github/workflows/ci.yml` job `e2e` with `e2e-redis` (`Go E2E Redis`) and `e2e-dragonfly` (`Go E2E Dragonfly`). Redis job: Redis `:6379` + AUTH `:6381`, Redis LIVE env only. Dragonfly job: Dragonfly `:6380` + AUTH `:6382`, Dragonfly LIVE env only. Both: `go test -timeout 5m -count=1 -v ./...` without `-short` and without `-race`
- [x] 1.2 Leave `lint`, `test`, `race`, and `integration` unchanged

## 2. Shared live skip/run

- [x] 2.1 In `simpleredis/simpleredis_e2e_test.go`, `windowcounter/limiter_e2e_test.go`, and `tokenbucket/limiter_e2e_test.go`: `lookupLiveEngineAddrs` returns the set engines; drop `errLiveEngineOneAddr`. Skip only `-short` or both unset. Update `TestLookupLiveEngineAddrs` `onlyRedis` / `onlyDragonfly` to expect one engine and nil error. Keep `bothSet` as two engines

## 3. Docs

- [x] 3.1 Update `knowledge/devdocs/std_go_test-suites.md` and README Tests: Go E2E is two CI jobs; one-engine env is a one-engine run; avoid “fail if exactly one addr” / “Avoid: one-engine-only runs”
- [x] 3.2 Run `openspec validate split-go-e2e-engine-jobs --type change --strict`
