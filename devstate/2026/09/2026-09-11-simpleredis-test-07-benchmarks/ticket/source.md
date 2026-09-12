# test-07 — No benchmark or allocation guard existed

Finding: `simpleredisfixes/test-07-hot-path-benchmarks.md`
Index: `simpleredisfixes/README.md`

Land the finding's Still to do: CI allocation guards (allocs/op and B/op, not ns/op), large-value decode benchmark, and keep churn tests. Measurement files may already exist from the review. Do not wait for perf-01 to land before CI guards; if churn cannot become a hard cap assertion yet, record that later on explore.md and still ship the rest.

HARD REQUIREMENT: tests MUST run against both Redis and Dragonfly. Both are supported backends. Keep live verb coverage on both engines in CI (compose + Pester `/redis` `/dragonfly`). Lua 5.1-safe. Dragonfly KEYS required.

## Still to do (from finding)

- Turn the two churn measurements into assertions once perf-01 lands.
- Decide whether these run in CI. Allocation counts are stable enough to assert; wall-clock numbers on shared CI runners are not, so gate on `allocs/op` and `B/op`, not `ns/op`.
- Add a decode benchmark for a large value (100 KB `Set`/`Get`) — every measurement here uses small values, so the buffer-growth behaviour of the encode path is unmeasured.

## What was added during the review (may already exist as measurement files)

`simpleredis/bench_test.go`: end-to-end benches against the fake server, client-side encode/decode allocation guards, `BenchmarkGetParallel` dial metric, `TestConnectionChurnUnderLatency` and `TestConnectionChurnAcrossBursts` (currently `t.Logf` only).

`simpleredis/interpretedcost_test.go`: Yaegi unsafe variants, interpreted encode/conversion/`Get` benches, compiled pairs.

Two helpers were widened from `*testing.T` to `testing.TB` so benchmarks could reuse them: `startFakeRedis`, `writeGopathSimpleredis` / `writeGopathFile`. No production code was changed.
