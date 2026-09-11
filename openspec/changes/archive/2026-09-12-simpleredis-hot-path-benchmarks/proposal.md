## Why

Dest SimpleRedis has no compiled encode/decode/end-to-end benches, no 100 KB decode measurement, and no CI gate on `allocs/op` or `B/op`. `go test` does not run `Benchmark*` without `-bench`, so a helper on the encode path or buffer growth on a 100 KB bulk would not fail the suite.

## What Changes

- Land compiled encode/decode/end-to-end benches, Yaegi interpreted-cost measurements, two connection-churn tests (`t.Logf` only), and a 100 KB canned bulk decode plus 100 KB SET encode.
- CI allocation guards are `TestXxx` functions that call `testing.Benchmark` with `ReportAllocs` and fail on over-budget `AllocsPerOp` / `AllocedBytesPerOp` (Go 1.21 ceilings). Do not add `-bench` to `ci.yml`. Do not gate `ns/op`.
- Widen `startFakeRedis`, `writeGopathSimpleredis`, and `writeGopathFile` to `testing.TB`. No production `simpleredis.go` change.
- Keep compose Redis and Dragonfly, the CI `integration` job, and Pester `/redis` `/dragonfly` every-verb coverage. Eval stays Lua 5.1-safe with KEYS declared.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: CI `go test` without `-bench` SHALL fail when client-side encode/decode (including 100 KB) exceeds the Go 1.21 `allocs/op` or `B/op` ceiling. Pester Redis and Dragonfly every-verb scenarios stay; do not skip either engine.

## Impact

- `simpleredis/simpleredis_test.go` (`startFakeRedis` signature), `simpleredis/yaegi_test.go` (GOPATH helpers), new `simpleredis/bench_test.go` and `simpleredis/interpretedcost_test.go`.
- `.github/workflows/ci.yml` only if a Test entry must be named; do not drop Redis/Dragonfly services or the integration job; do not add `-bench`.
- `scripts/integration-tests.Tests.ps1`, `docker-compose.yml`, `e2e/simpleredisprobe/` — keep `/redis` and `/dragonfly`; do not remove.
- Main spec `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` after the tests exist (devdocsimpact / implement). No `simpleredis.go` production change. No perf-01 pool cap.
