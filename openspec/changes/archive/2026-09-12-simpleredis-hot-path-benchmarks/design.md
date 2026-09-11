## Context

Dest `simpleredis/` has `simpleredis.go`, `simpleredis_test.go`, `yaegi_test.go`, and `LICENSE` only. `startFakeRedis` and Yaegi GOPATH helpers take `*testing.T`. CI `go test` has no `-bench` (Go 1.21). See proposal.md for why. Proceed policies: `devstate/explore.md`. Live engines stay compose + Pester `/redis` `/dragonfly` (`std_go_simpleredis_resp-commands`).

## Goals / Non-Goals

**Goals:**
- Client-side encode/decode alloc/B/op gates that `go test` without `-bench` runs.
- 100 KB canned bulk decode and 100 KB SET encode.
- Land reviewer measurement files (benches, interpreted-cost, churn `t.Logf`).
- Keep Redis and Dragonfly verb coverage.

**Non-Goals:**
- Production `simpleredis.go` change; perf-01 pool cap assertions; `-bench` or benchstat in CI; `ns/op` gates; live-engine alloc tests.

## Decisions

1. **CI gates are `TestXxx` + `testing.Benchmark`.** Inner funcs call `b.ReportAllocs()` and assert `AllocsPerOp` / `AllocedBytesPerOp` only. Alternative: `go test -bench` or benchstat — rejected; dest CI has no `-bench`, and `AllocsPerRun` cannot guard B/op. `b.Loop()` is not in Go 1.21; benches stay `for i := 0; i < b.N; i++`.

2. **Ceilings measured on Go 1.21.** Slack: +1 allocs/op; B/op = measured + 64 bytes (or 20% if larger), documented in the Test. Do not copy Go 1.25.6 windows/amd64 numbers into CI.

3. **100 KB fixture is canned RESP.** Decode: `$102400` through `readReply` via `repeatReader`. Encode: 100 KB SET to `io.Discard`. Size `100*1024`. Alternative: live Set-then-Get — rejected; engine variance would make B/op flaky.

4. **Alloc guards do not dial engines.** Fake canned RESP for alloc/B/op. Keep compose services, integration job, and Pester `/redis` `/dragonfly`. Do not add `SIMPLE_REDIS_LIVE_*` alloc tests.

5. **Widen helpers to `testing.TB`.** `startFakeRedis`, `writeGopathSimpleredis`, `writeGopathFile`. Existing `*testing.T` callers keep compiling. Alternative: duplicate helpers — rejected; consume before produce.

6. **Land `interpretedcost_test.go`.** `TestYaegiUnsafeVariants` runs in CI. Yaegi `Benchmark*` stay local measurement, not CI alloc gates.

7. **Churn stays `t.Logf`.** `TestConnectionChurnUnderLatency` and `TestConnectionChurnAcrossBursts` do not `t.Fatal` on dial count. perf-01 owns a later cap assertion.

## Risks / Trade-offs

- [Go 1.21 vs local 1.25.6 alloc numbers] → Mitigation: measure ceilings on 1.21 during implement; slack as Decision 2.
- [CI runner noise on ns/op] → Mitigation: never assert `NsPerOp`.
- [Someone drops Dragonfly while adding benches] → Mitigation: spec scenario plus tasks that keep Pester routes; do not edit compose/Pester except to keep them.

## Migration Plan

Test-only. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
