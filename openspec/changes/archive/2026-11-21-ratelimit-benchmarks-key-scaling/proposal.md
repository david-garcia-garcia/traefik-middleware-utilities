## Why

The two rate limiters (`tokenbucket/`, `windowcounter/`) run both compiled into a custom Traefik build and interpreted by Yaegi as a Traefik plugin, yet the repo has no benchmark for either limiter in either mode, and the README does not compare them with each other or with Traefik's own rate limiters. The probe numbers gathered during explore also show two costs that grow with the number of keys: admitting a new source in `tokenbucket.Memory` walks the whole map under the global mutex (about 0.8 ms with 60k sources, 1.6 ms with the table full), and the buffered `windowcounter` flush sends one EVAL per dirty key in series (about N × RTT), so the real sync interval stretches with key count. The requester asked to fix both in this change without changing observable behavior.

## What Changes

- Add Go benchmarks for `tokenbucket.NewMemory`, `tokenbucket.NewRedis`, and `windowcounter` with `sync_rate` 0 and `sync_rate` > 0, each compiled and interpreted under Yaegi v0.16.1 (pattern of `simpleredis/interpretedcost_test.go` `BenchmarkYaegiGet`), against the in-process fake Redis. Hot key and 10,000 rotating keys; `tokenbucket.Memory` also "new key at full table (65536)"; `windowcounter` also a direct flush of 1,000 dirty keys. `b.RunParallel` variants compiled only. A `redis-cmds/op` metric on every fake-backed benchmark.
- Optional live benchmarks (compiled only) for both packages that reuse `TOKENBUCKET_LIVE_*` / `WINDOWCOUNTER_LIVE_*` and skip under `-short` or when both addresses are unset. No Yaegi live benchmarks.
- Test-only: each package's fake Redis gains an opt-in script-digest cache (off by default, turned on only by benchmarks) and a command counter, so fake numbers represent a Redis with a warm script cache. Test helpers used by benchmarks widen from `*testing.T` to `testing.TB`.
- `tokenbucket.Memory`: admitting a new source becomes O(1). Sources are kept in an intrusive list ordered by expiry; expired sources are removed from the front; at the 65536 cap with nothing expired, the front (least recently used) source is removed. Same cap, expired-first, an evicted source comes back as a full bucket, and admit/wait/deny decisions of stored sources do not change.
- `windowcounter` buffered flush: one EVAL per batch of up to 256 dirty keys instead of one per key. The batch script wraps each key's writes in `redis.pcall` and returns one slot per key, so each key keeps today's outcome (applied → leaves pending; failed → keeps its delta and is retried; first error stored; Take/Peek still return a nil error during an outage). No double count of applied keys on retry.
- README: new comparison section (algorithm, where state lives, Redis round trips per request, pros and cons, scalability across Traefik instances, key count and Redis outage) with a benchmark table from this code (command, machine, Go version, `-count 5` median) and a qualitative comparison with Traefik Proxy `RateLimit` and Traefik Hub `DistributedRateLimit` limited to documented facts. The two optimized rows carry a "before (commit `<sha>`)" column. The intro and `## Yaegi` state that the code runs compiled and under Yaegi.
- Devdocs: one benchmark usage line in `std_go_tokenbucket.md`, `std_go_windowcounter.md`, and `std_go_test-suites.md`; the batched flush and the source cap in the respective packets; `std_go_simpleredis.md` no longer says Yaegi "is not the speed target".
- No public API change, no new dependency, `simpleredis/` production code unchanged, CI unchanged (benchmarks compile but do not run).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `std_go_tokenbucket_allow`: adds a requirement that pins the in-memory source cap (65536), removal of expired sources before any live source, and that an evicted source restarts as a full bucket. Which live source is evicted stays unspecified.
- `std_go_windowcounter_sync-flush`: the buffered flush sends batches of keys per EVAL (at most 1024 keys) with a per-key outcome instead of one EVAL per key; the flush reply parse rule applies per key slot.

## Impact

- **Code**: `tokenbucket/memory.go` (expiry list, eviction), `windowcounter/limiter.go` (batched flush script and result handling).
- **Tests**: new `bench_test.go`, `bench_yaegi_test.go`, `bench_e2e_test.go` in `tokenbucket/` and `windowcounter/`; fake Redis files of both packages (opt-in digest cache, command counter, batch flush script, per-key failure injection in `windowcounter`); test helpers widened to `testing.TB`; new cap/eviction test for `Memory`; new partial-batch and in-flight tests for the flush; `allow_wait_duration_overflow_test.go` inserts its entry through the list helper.
- **Docs**: `README.md` (new comparison section, intro, `## Yaegi`), `knowledge/devdocs/std_go_tokenbucket.md`, `std_go_windowcounter.md`, `std_go_test-suites.md`, `std_go_simpleredis.md`.
- **Engines**: the batched flush touches several keys in one EVAL. Redis standalone and Dragonfly are supported (all keys declared in `KEYS`); a cluster engine would answer CROSSSLOT, but SimpleRedis does not support Redis Cluster today either.
- **Unchanged**: public API of both packages, `simpleredis/` production code, Traefik Hub semantics (out of scope), `go.mod`.
