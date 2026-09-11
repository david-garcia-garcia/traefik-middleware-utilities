## 1. Test helpers

- [x] 1.1 Widen `startFakeRedis` in `simpleredis/simpleredis_test.go` from `*testing.T` to `testing.TB`
- [x] 1.2 Widen `writeGopathSimpleredis` and `writeGopathFile` in `simpleredis/yaegi_test.go` to `testing.TB`
- [x] 1.3 Run `go test ./simpleredis/...` until existing compiled and Yaegi tests still pass (no `simpleredis.go` change)

## 2. Measurement files

- [x] 2.1 Add `simpleredis/bench_test.go`: end-to-end fake-server benches (Get, MGet10, Incr, Eval, GetParallel), client-side encode/decode (Get, Eval, bulk, array10, integer) with `b.ReportAllocs()` and `for i := 0; i < b.N; i++`
- [x] 2.2 Add 100 KB canned `$102400` bulk decode via `repeatReader` + `readReply` and 100 KB SET encode to `io.Discard` (`100*1024`); not a live engine round-trip
- [x] 2.3 Add `TestConnectionChurnUnderLatency` and `TestConnectionChurnAcrossBursts` as `t.Logf` only (no `t.Fatal` on dial count)
- [x] 2.4 Add `simpleredis/interpretedcost_test.go` including `TestYaegiUnsafeVariants`; Yaegi `Benchmark*` stay local measurement, not CI alloc gates

## 3. CI allocation guards

- [x] 3.1 Add `TestXxx` functions that call `testing.Benchmark` on the client-side encode/decode loops (Get/Eval encode, bulk/array/integer decode, 100 KB) with `b.ReportAllocs()`
- [x] 3.2 Measure Go 1.21 `AllocsPerOp` / `AllocedBytesPerOp`; set ceilings with slack (+1 allocs/op; B/op = measured + 64 bytes or 20% if larger); do not copy Windows 1.25.6 numbers; do not assert `NsPerOp`
- [x] 3.3 Run `go test -timeout 2m -count=1 ./simpleredis/...` without `-bench` until the guards pass; confirm over-budget would fail; do not add `-bench` to `.github/workflows/ci.yml`

## 4. Live Redis and Dragonfly

- [x] 4.1 Confirm `docker-compose.yml` still has `redis:7-alpine` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`, and Pester `scripts/integration-tests.Tests.ps1` still asserts Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval on `/redis` and `/dragonfly` with Lua 5.1-safe KEYS; do not skip either engine
- [x] 4.2 Confirm CI `test` job still starts Redis and Dragonfly services and the `integration` job still runs; do not drop those jobs or add `SIMPLE_REDIS_LIVE_*` alloc tests

## 5. Specs

- [x] 5.1 Confirm the change delta `std_go_simpleredis_resp-commands` matches the landed tests
- [x] 5.2 Run `openspec validate --change simpleredis-hot-path-benchmarks --strict`
