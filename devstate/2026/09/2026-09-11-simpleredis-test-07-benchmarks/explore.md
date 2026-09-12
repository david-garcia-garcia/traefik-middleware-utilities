# Explore
IssueKey: 2026-09-11-simpleredis-test-07-benchmarks

Verdict: in progress

## Concepts

- **SimpleRedis** — dest `simpleredis/simpleredis.go` at `origin/master` `7dc4b051`. Stdlib pooled TCP RESP. `writeCommand` (`:310`) encodes; `readReply` / `readBulk` (`:329`, `:389`) decode. `readBulk` always `make([]byte, length+2)`. Idle pool cap `maxIdleConns = 8`. No `go-redis`, miniredis, TLS.
- **Fake Redis** — `startFakeRedis` in `simpleredis/simpleredis_test.go:28` is `func(*testing.T, map[string]string)`. 16 call sites in `simpleredis_test.go` plus 2 in `yaegi_test.go` (searched `simpleredis/` for `startFakeRedis(`). Yaegi GOPATH helpers `writeGopathSimpleredis` / `writeGopathFile` are also `*testing.T` (`yaegi_test.go:60`, `:96`).
- **Dest measurement gap** — `simpleredis/` on this worktree is `simpleredis.go`, `simpleredis_test.go`, `yaegi_test.go`, `LICENSE` only. No `bench_test.go`, no `interpretedcost_test.go`. Reviewer copies exist only on the other checkout `d:\repositories\traefik-middleware-utilities\simpleredis\bench_test.go` and `interpretedcost_test.go` (not dest). This ticket lands them.
- **CI** — `.github/workflows/ci.yml` `test` job: `go test -timeout 2m -count=1 -v ./...` (line 65), Go 1.21, services `redis:7-alpine` `:6379` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` `:6380`. No `-bench`. `integration` job runs `./Test-Integration.ps1`.
- **Live verbs** — compose PathPrefix `/redis` and `/dragonfly`. Pester `scripts/integration-tests.Tests.ps1` asserts Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval on both. Probe Eval is `kongIncrbyExpireatScript` with `KEYS[1]` (`e2e/simpleredisprobe/plugin.go:17`). Dragonfly forbids undeclared keys and lacks `table.maxn` (`knowledge/research/ext_dragonfly_eval/`).
- **Go testing (official)** — `BenchmarkXxx(*testing.B)` runs only when `go test -bench` is set ([pkg.go.dev/testing](https://pkg.go.dev/testing#hdr-Benchmarks)). `testing.AllocsPerRun` returns allocation **count** only, not B/op ([AllocsPerRun](https://pkg.go.dev/testing#AllocsPerRun)). `testing.Benchmark` returns `BenchmarkResult` with `AllocsPerOp` and `AllocedBytesPerOp` from inside a `TestXxx` without `-bench`, if the inner `b.ReportAllocs()` ran ([Benchmark](https://pkg.go.dev/testing#Benchmark), [ReportAllocs](https://pkg.go.dev/testing#B.ReportAllocs)). `b.Loop()` is not in Go 1.21; benches stay `for i := 0; i < b.N; i++`.
- **Usage** — `knowledge/devdocs/std_go_simpleredis.md` proves with `go test ./simpleredis/...` and `./Test-Integration.ps1`. No allocation-guard instruction yet (produce after the tests exist).
- **Specs** — `std_go_simpleredis_resp-commands` owns Pester every-verb Redis/Dragonfly and Lua 5.1-safe KEYS. `std_go_simpleredis_tcp-session` owns Init/pool/Close. No alloc-guard requirement. No active OpenSpec change (`openspec list` empty).
- **Out of scope** — perf-01 pool cap, perf-05 EVALSHA, perf-06/07/08, ns/op CI gates, Init/Close/pool/TLS/go-redis/miniredis, dropping Pester routes.

No identity reconstruction (client address, user, tenant, Host).

```
  CI test job                         CI integration job
  go test ./...  (no -bench)          ./Test-Integration.ps1
        │                                      │
        ├─ TestXxx  ← alloc guards live here   ├─ GET /redis    (Redis)
        ├─ BenchmarkXxx  ← NOT run today       └─ GET /dragonfly (Dragonfly)
        └─ live windowcounter/tokenbucket
           already dial both engines
```

### Reproduction

Claimed gap **reproduced** (absence, not a crashing test).

Worktree `d:\repositories\wt-modsec-2026-09-11-simpleredis-test-07-benchmarks`, HEAD `e107867` (prepare only; dest `7dc4b05`):

1. `go test -timeout 2m -count=1 ./simpleredis/...` — **ok** in 2.174s (local `go1.25.6 windows/amd64`). Suite has no `Benchmark*` and no alloc assertion, so it cannot fail an encode/decode allocation regression.
2. Dest `ci.yml:65` has no `-bench`. Official testing docs: `BenchmarkXxx` is not executed without that flag.
3. `startFakeRedis` on dest is `*testing.T`. Reviewer `bench_test.go` calls `startFakeRedis(b, …)` — will not compile until the helper is `testing.TB`.

## Decisions

- Land reviewer `simpleredis/bench_test.go` (end-to-end fake-server benches, client-side encode/decode benches, `BenchmarkGetParallel`, churn tests) and `interpretedcost_test.go`. Dest does not have them; do not assume it does.
- Widen `startFakeRedis`, `writeGopathSimpleredis`, and `writeGopathFile` to `testing.TB`. Existing 18 `*testing.T` callers keep compiling (`*testing.T` is a `testing.TB`). No `simpleredis.go` production change.
- CI allocation guards are `TestXxx` functions that call `testing.Benchmark` on the **client-side** encode/decode loops (Get/Eval encode, bulk/array/integer decode, plus 100 KB). Each inner func calls `b.ReportAllocs()` and does not assert `NsPerOp`. Fail when `AllocsPerOp` or `AllocedBytesPerOp` exceeds the Go 1.21 ceiling. Do not add `-bench` to `ci.yml`. Do not add benchstat. `testing.AllocsPerRun` cannot guard B/op.
- Keep `Benchmark*` for local `go test -bench` (trend line). They are not the CI gate.
- 100 KB fixture: canned bulk GET of `100*1024` bytes for decode (`repeatReader` + `readReply`), plus encode of a 100 KB SET to `io.Discard` (buffer growth on `writeCommand`). Not a live engine round-trip.
- Churn tests ship as `t.Logf` only. They cannot become a hard pool-cap assertion in this ticket: perf-01 is Out of scope. Still ship them.
- Live Redis and Dragonfly: keep compose services, `integration` job, and Pester `/redis` `/dragonfly` verb coverage. Lua stays 5.1-safe with KEYS declared. Do not add `SIMPLE_REDIS_LIVE_*` alloc tests. Do not drop or skip either engine.
- Spec: propose folds one alloc-guard requirement into existing `std_go_simpleredis_resp-commands` (CI `go test` without `-bench` fails on over-budget allocs/op or B/op; Pester Redis/Dragonfly scenarios unchanged). Run FindSpecHost at propose; if that verdict is `new`, use a legal 4th part under `std_go_simpleredis_*` — do not invent a parallel client.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` gets an allocation-guard line after the tests exist (devdocsimpact / implement). Language is already enough.
- No `knowledge/research/` write: Redis/Dragonfly facts already in `ext_dragonfly_eval` and `ext_dragonfly_container-image`. Go testing facts cited from pkg.go.dev in this file.

## Open questions

- Q: Guard mechanism in CI — `testing.AllocsPerRun` Test functions vs `go test -bench` with a golden vs benchstat?
  Rank: additive asked — new Test functions this change creates; Unknowns line 1 and Desired "CI allocation guards on allocs/op and B/op, not ns/op"
  Decision: assumed — `TestXxx` + `testing.Benchmark` + `b.ReportAllocs()` asserting `AllocsPerOp` and `AllocedBytesPerOp` only. No `-bench` on `ci.yml`. No benchstat. `AllocsPerRun` alone is count-only ([pkg.go.dev/testing#AllocsPerRun](https://pkg.go.dev/testing#AllocsPerRun)).
  By: explore

- Q: Must new allocation-guard tests themselves dial live Redis and Dragonfly, or are fake-server benches plus existing Pester enough?
  Rank: additive asked — Unknowns line 2; HARD REQUIREMENT names compose + Pester `/redis` `/dragonfly`
  Decision: assumed — fake canned RESP for alloc/B/op; keep compose + Pester `/redis` and `/dragonfly` as the live verb proof. Do not add `SIMPLE_REDIS_LIVE_*`. Do not drop CI Redis/Dragonfly services or the integration job. Engine variance would make B/op flaky and is not the encode/decode job.
  By: explore

- Q: Numeric `allocs/op` and `B/op` budgets — finding numbers are Go 1.25.6 windows/amd64; CI is Go 1.21 ubuntu-latest?
  Rank: additive asked — Unknowns line 3; Desired names those two metrics as the gate
  Decision: assumed — measure ceilings during implement on Go 1.21 (module and CI). Local `go1.25.6 windows/amd64` here is the finding's platform, not the gate. Slack: +1 allocs/op; B/op = measured + 64 bytes (or 20% if larger), documented in the Test. Do not copy Windows 1.25.6 numbers into CI.
  By: explore

- Q: Is `interpretedcost_test.go` in this ticket's ship set?
  Rank: additive asked — Unknowns line 4; Desired "Land the finding's measurement files"; finding lists that file under What was added
  Decision: assumed — yes, land it. Widen Yaegi GOPATH helpers to `testing.TB`. `TestYaegiUnsafeVariants` runs in CI as a Test. Yaegi `Benchmark*` stay local measurement, not CI alloc gates.
  By: explore

- Q: Exact 100 KB decode fixture shape (bulk GET reply vs full Set-then-Get round trip)?
  Rank: additive asked — Unknowns line 5; Desired "a 100 KB Set/Get decode benchmark"
  Decision: assumed — canned `$102400` bulk GET through `readReply` (client-only, same `repeatReader` as `BenchmarkDecodeBulk`) plus a 100 KB SET encode to `io.Discard`. Size `100*1024`. Not live Redis/Dragonfly.
  By: explore

- Q: Can the two churn tests become a hard pool-cap assertion in this ticket?
  Rank: additive asked — Desired "If they cannot become a hard pool-cap assertion yet, record that on explore.md and still ship the rest"; Out of scope names perf-01
  Decision: resolved — they cannot. Keep `TestConnectionChurnUnderLatency` and `TestConnectionChurnAcrossBursts` as `t.Logf` measurements. Do not `t.Fatal` on dial count until a later perf-01 change owns the cap. Still ship CI alloc guards, the 100 KB decode, and the churn tests.
  By: explore
