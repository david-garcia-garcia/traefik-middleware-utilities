## 1. CI job split

- [x] 1.1 Move Redis and Dragonfly services plus all `*_LIVE_*` env from `test` to a new job `e2e` named `Go E2E` in `.github/workflows/ci.yml`. Unit `test` has no services and runs `go test -short -timeout 2m -count=1 -v ./...`. E2e runs `go test -timeout 5m -count=1 -v ./...` without `-short`
- [x] 1.2 Keep `lint` and `integration` (Pester) unchanged. Do not add engine images

## 2. Shared live skip/fail

- [x] 2.1 Shared `liveEngineAddrs` / `runForEachLiveEngine`: skip when `-short` or both addrs unset; `t.Fatal` when exactly one addr is set; run every case on both engines when both are set. Helpers in `{package}_e2e_test.go` or `{domain}_e2e_test.go`

## 3. SimpleRedis live coverage

- [x] 3.1 Domain e2e files: `commands_e2e_test.go`, `commands_eval_e2e_test.go`, `commands_msetex_e2e_test.go`, `pool_e2e_test.go`, harness `simpleredis_e2e_test.go`. Engine-success Get/Set/Del/MGet/Incr/Expire/Eval/EVALSHA/NOSCRIPT-after-SCRIPT-FLUSH, pool wait, MSetEX, CLIENT KILL, SELECT 99, WRONGPASS on `SIMPLEREDIS_LIVE_*_AUTH`. Keys from `t.Name()`. Keep fake-TCP for malformed RESP and AUTH-class prefixes dest engines do not emit
- [x] 3.2 Add `yaegi_e2e_test.go` (`TestYaegiLive_RedisAndDragonfly`): compiled test owns skip; interpreted probe `LiveVerbs`. GOPATH, stdlib only, `useunsafe` false, no Traefik
- [x] 3.3 Run `go test -short ./simpleredis/...` until unit tests pass and e2e tests skip

## 4. Limiter live coverage

- [x] 4.1 `windowcounter/limiter_e2e_test.go` and `limiter_yaegi_e2e_test.go`: Peek denied-then-slides, buffered Peek, expire-on-first-hit plus dest scenarios, both engines
- [x] 4.2 `tokenbucket/limiter_e2e_test.go` and `limiter_yaegi_e2e_test.go`: refund-when-wait-exceeds-maxDelay plus dest scenarios, both engines
- [x] 4.3 Run `go test -short ./windowcounter/... ./tokenbucket/...` until unit tests pass and e2e tests skip

## 5. Docs

- [x] 5.1 Add `knowledge/devdocs/std_go_test-suites.md` (lint, unit Go, Go E2E, Pester, `{domain}_e2e_test.go` naming). Index row on `index_std_go.md`. Point prove-with on `std_go_simpleredis.md`, `std_go_windowcounter.md`, `std_go_tokenbucket.md` at that catalog. Update README Tests
- [x] 5.2 Run `openspec validate add-go-e2e-live-backends --type change --strict`
