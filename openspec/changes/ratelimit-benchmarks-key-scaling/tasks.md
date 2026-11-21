## 1. Benchmark harness (test-only, commit 1)

- [ ] 1.1 Widen `startTestFakeRedis`, `writeGopathTokenbucket`, `writeGopathWindowcounter`, `copyNonTestGo`, `callerDir`, `writeGopathFile` to `testing.TB` in both packages; verify `go vet ./tokenbucket/ ./windowcounter/` is clean with no caller edits
- [ ] 1.2 Add the opt-in script-digest cache (`scriptCache`, `loadedScripts`) and a served-command counter to `tokenbucket/fake_redis_test.go` and `windowcounter/fake_redis_test.go` (design D2), off by default; verify `go test -short -count=1 ./tokenbucket/ ./windowcounter/` passes unchanged, including every `lastEvalCommand()` assertion

## 2. Token bucket benchmarks (commit 1)

- [ ] 2.1 Add `tokenbucket/bench_test.go`: `Memory` `hot-key`, `rotating-10k`, `new-key-full-table`, `hot-key-parallel`; `Redis` `hot-key`, `rotating-10k`, `hot-key-parallel` against the fake with the cache on; `ReportAllocs` and `redis-cmds/op` (design D1); verify `go test -run '^$' -bench . -benchtime 100x ./tokenbucket/` runs every sub-benchmark
- [ ] 2.2 Add `tokenbucket/bench_yaegi_test.go`: interpreted `Memory` hot key and `Redis` hot key (probe `Loop(count)`, warm-up `Eval`, timed `Eval(b.N)`, no unsafe); verify it runs with `-bench Yaegi -benchtime 100x`
- [ ] 2.3 Add `tokenbucket/bench_e2e_test.go`: compiled live `tokenbucket.Redis` benchmarks (`hot-key`, `rotating-10k`) per engine via `lookupLiveEngineAddrs`, unique key prefix per run, skip under `-short` or both `TOKENBUCKET_LIVE_*` unset; verify `go test -short -run '^$' -bench Live ./tokenbucket/` skips

## 3. Window counter benchmarks (commit 1)

- [ ] 3.1 Add `windowcounter/bench_test.go`: `sync_rate` 0 and 20 ms, each `hot-key`, `rotating-10k`, `hot-key-parallel`, plus `BenchmarkFlush` (1000 dirty keys, timer off, re-dirty outside the timed section); `ReportAllocs` and `redis-cmds/op`; `Close` every limiter; verify `go test -run '^$' -bench . -benchtime 100x ./windowcounter/` runs every sub-benchmark and returns
- [ ] 3.2 Add `windowcounter/bench_yaegi_test.go`: interpreted `sync_rate` 0 and 20 ms hot key against the fake with the cache on; verify it runs with `-bench Yaegi -benchtime 100x` and does not hang on `Close`
- [ ] 3.3 Add `windowcounter/bench_e2e_test.go`: compiled live benchmarks per engine (`sync_rate` 0 and 20 ms), unique key prefix, skip under `-short` or both `WINDOWCOUNTER_LIVE_*` unset; verify `go test -short -run '^$' -bench Live ./windowcounter/` skips

## 4. Baseline (commit 1)

- [ ] 4.1 Run `go test -short -count=1 ./...` and `golangci-lint run` (v1.63.4); both green
- [ ] 4.2 Run `go test -run '^$' -bench . -benchtime 1s -count 5 ./tokenbucket/ ./windowcounter/` on the reference machine (Core Ultra 7 265K, Windows 11, go1.25.6), keep the raw output, compute medians; checkpoint commit 1 with benchmarks only and record its SHA for the README "before" column

## 5. `tokenbucket.Memory` O(1) admission (commit 2)

- [ ] 5.1 Add the cap/eviction test first (full table with expired and live sources → expired removed, live kept; full table of live sources → `len` stays 65536, new key starts at `burst - 1`; evicted key comes back full); verify it passes on the commit-1 code
- [ ] 5.2 Implement the intrusive expiry-ordered list in `tokenbucket/memory.go` (design D3): `key`, `prev`, `next` on `memEntry`, `head`/`tail` on `Memory`, move-to-tail on every `Allow`, pop expired heads on a new source, pop the head at the cap; remove `dropExpired`/`dropOne`; no `container/list`, no `any`
- [ ] 5.3 Route `allow_wait_duration_overflow_test.go`'s hand-inserted `memEntry` through the link helper; verify `go test -short -count=1 ./tokenbucket/` passes with no other test edits (including `TestMemory_*`, `TestRepro_*`, `allow_maxdelay*`, Yaegi tests) and the 5.1 test still passes
- [ ] 5.4 Add the source cap and "removes expired sources first, victim unspecified" gotcha to `knowledge/devdocs/std_go_tokenbucket.md`; verify it matches the `std_go_tokenbucket_allow` delta

## 6. `windowcounter` batched flush (commit 2)

- [ ] 6.1 Teach `windowcounter/fake_redis_test.go` the batch script (numkeys, KEYS, ARGV pairs, array reply, `EXPIREAT` recorded for new keys) and add per-key INCRBY failure injection; replace the single-key `flushScript` branch
- [ ] 6.2 Replace `flushScript` with the batch script and batch the flush in `flushPendingLocked` (design D4: K = 256, per-key `redis.pcall`, `err:` slot marker, slot-count check, first error stored, `l.mu` dropped per batch); keep `flushPending` callers (`flushAfterTick`, `Sleep`, `Close`) unchanged
- [ ] 6.3 Add tests: (i) one key fails inside a batch while the others apply; (ii) a Take during an in-flight batch keeps its hit; (iii) applied keys of a partially failed batch are not re-sent; slot-count mismatch fails every key of that batch; 1000 dirty keys take fewer EVALs than keys; verify they pass
- [ ] 6.4 Verify the existing flush/outage tests pass unchanged: `go test -short -count=1 ./windowcounter/` (including `TestBuffered_*`, `TestTake_Buffered*`, `TestPeek_Buffered*`, `repro_buffered_outage_test.go`, `repro_stale_previous_test.go`, `repro_lock_during_get_test.go`, `repro_sleep_wake_race_test.go`, `yaegi_flush_stop_test.go`)
- [ ] 6.5 Add a live multi-key flush case to `windowcounter/limiter_e2e_test.go` (several keys in one EVAL, every delta applied once) so `e2e-redis` and `e2e-dragonfly` prove it; verify locally against at least one engine or rely on the CI jobs
- [ ] 6.6 Update `knowledge/devdocs/std_go_windowcounter.md`: batched flush (K keys per EVAL, per-key outcome), the lost-reply double-apply now per batch, and the cluster/CROSSSLOT limitation; verify it matches the `std_go_windowcounter_sync-flush` delta

## 7. After numbers, README, devdocs (commit 2)

- [ ] 7.1 Re-run the 4.2 command on the same machine; record both series and both SHAs for the PR card
- [ ] 7.2 Add the README comparison section and benchmark table (design D6): final medians, "before (commit `<sha>`)" column on `new-key-full-table` and `BenchmarkFlush`, command, machine, Go version, `go 1.21` floor note, fake = warm script cache, Traefik Proxy / Hub comparison limited to the two research folders; verify every number in the table appears in the recorded output
- [ ] 7.3 Update the README intro (clause about the compiled build) and rewrite `## Yaegi` (compiled and under Yaegi); verify no README sentence still says the code is only interpreted
- [ ] 7.4 Fix the `knowledge/devdocs/std_go_simpleredis.md` sentence ("not the speed target") to "runs compiled and under Yaegi" while keeping the encoder rule; add the benchmark usage line to `std_go_tokenbucket.md`, `std_go_windowcounter.md`, and `std_go_test-suites.md` (not run in CI, how to run, live variables)

## 8. Integration

- [ ] 8.1 `go test -short -count=1 ./...`, `golangci-lint run`, and `go test -run '^$' -bench . -benchtime 100x ./tokenbucket/ ./windowcounter/` all green; CI `e2e-redis` and `e2e-dragonfly` green
- [ ] 8.2 Close the two `take` rows in `devstate/.../issues.md` (`[x]`, `Taken:`) and delete `knowledge/debt/2026-11-21-tokenbucket-memory-new-source-scan.md` and `knowledge/debt/2026-11-21-windowcounter-serial-flush.md`

## Workflow follow-up

- Archive the change after code review; the deltas fold into `std_go_tokenbucket_allow` and `std_go_windowcounter_sync-flush`.
