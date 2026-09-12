## 1. CI job split

- [ ] 1.1 Move Redis and Dragonfly services plus all `*_LIVE_*` env from `test` to a new job `e2e` named `Go E2E` in `.github/workflows/ci.yml`. Unit `test` has no services and runs `go test -short -timeout 2m -count=1 -v ./...`. E2e runs `go test -timeout 5m -count=1 -v ./...` without `-short`
- [ ] 1.2 Keep `lint` and `integration` (Pester) unchanged. Do not add engine images

## 2. Shared live skip/fail

- [ ] 2.1 In `simpleredis/live_test.go`, `windowcounter/live_test.go`, `tokenbucket/live_test.go`, and existing `TestYaegiLive_*`: skip when `-short` or both addrs unset; `t.Fatal` when exactly one addr is set; run every case on both engines when both are set

## 3. SimpleRedis live coverage

- [ ] 3.1 Expand `simpleredis/live_test.go` with engine-success Get/Set/Del/MGet/Incr/IncrBy/Expire/Eval/EVALSHA/NOSCRIPT-after-SCRIPT-FLUSH plus existing pool wait, MSetEX, and CLIENT KILL. Keys from `t.Name()`. Keep fake-TCP for malformed/AUTH
- [ ] 3.2 Add `TestYaegiLive_RedisAndDragonfly` in `simpleredis/yaegi_test.go` matching windowcounter: compiled test owns skip; interpreted probe calls New/Get/Set/Del/Incr/Eval/MSetEX. GOPATH, stdlib only, `useunsafe` false, no Traefik
- [ ] 3.3 Run `go test -short ./simpleredis/...` until unit tests pass and live tests skip

## 4. Limiter live coverage

- [ ] 4.1 Expand `windowcounter/live_test.go` and Yaegi live with Peek denied-then-slides, buffered Peek, and expire-on-first-hit on both engines
- [ ] 4.2 Expand `tokenbucket/live_test.go` and Yaegi live with refund-when-wait-exceeds-maxDelay on both engines
- [ ] 4.3 Run `go test -short ./windowcounter/... ./tokenbucket/...` until unit tests pass and live tests skip

## 5. Docs

- [ ] 5.1 Add `knowledge/devdocs/std_go_test-suites.md` (lint, unit Go, Go E2E, Pester). Index row on `index_std_go.md`. Point prove-with on `std_go_simpleredis.md`, `std_go_windowcounter.md`, `std_go_tokenbucket.md` at that catalog. Update README Tests
- [ ] 5.2 Run `openspec validate add-go-e2e-live-backends --type change --strict`
