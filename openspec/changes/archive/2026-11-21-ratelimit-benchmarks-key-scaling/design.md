## Context

See `proposal.md` — Why. Explore measured both limiters with a throwaway probe (`devstate/.../explore.md` → Concepts → Reproducción) and chose the approach for every open point; this design fixes the remaining shapes.

Current state that constrains the approach:

- `tokenbucket/memory.go`: `Allow` holds `m.mu`, deletes a present-but-expired entry, and for every new source calls `dropExpired` (full map walk) and, at `maxMemorySources` (65536), `dropOne` (another walk, then deletes the first key of map iteration order). `expireAt = now + ttl`, with `ttl` fixed per limiter and `now` read after `m.mu` is held.
- `windowcounter/limiter.go`: `flushPendingLocked` snapshots every window with `localDelta > 0 && flushDelta == 0` under `l.mu`, then for each one drops `l.mu`, sends one `Eval` of `flushScript` (`EXISTS`, `INCRBY`, `EXPIREAT`-if-new), re-locks and applies the result. First error → `lastOutageErr` / `lastOutageAt`. Called from `flushAfterTick` (`time.AfterFunc`), `Sleep`, `Close`.
- `simpleredis.Eval` returns a flat array of bulk strings or integers; nested tables and `{err=...}` inside an array are `redis:unsupported-reply`. There is no pipeline or MULTI/EXEC and this change does not add one.
- Both fake Redis servers (`tokenbucket/fake_redis_test.go`, `windowcounter/fake_redis_test.go`) answer every EVALSHA with NOSCRIPT, so each `Eval` costs two round trips; tests read `lastEvalCommand()` and match `args[1] == flushScript`.
- Yaegi v0.16.1 (same as Traefik v3.7.11): values passed through `any` have lost methods before; interpreted `go` + `select` can miss a close.

## Goals / Non-Goals

**Goals:**

- Reproducible compiled and Yaegi numbers for both limiters, with a before/after for the two optimizations taken from the same benchmarks and command.
- O(1) new-source admission in `tokenbucket.Memory` at any table size, same cap and decisions.
- ⌈N/K⌉ flush round trips instead of N, with today's per-key outcome.

**Non-Goals:**

- Running `-bench` in CI or adding performance thresholds.
- Optimizing `NewRedis`, exact mode (`sync_rate` 0), or `Peek`.
- Changing the 65536 cap, TTL rules, `minSyncRate`, or any public signature.
- Redis Cluster support (hash tags, CROSSSLOT splitting).
- Changing `simpleredis/` production code or the `std_go_simpleredis_resp-commands` encoder rule.

## Decisions

### D1. Benchmark files and matrix

Files per package, named by domain as `std_go_test-suites.md` prescribes (precedent `simpleredis/bench_test.go`, `backendbackoff/bench_test.go`):

| File | Mode | Backend |
|---|---|---|
| `tokenbucket/bench_test.go`, `windowcounter/bench_test.go` | compiled | fake Redis (digest cache on) / in-memory |
| `tokenbucket/bench_yaegi_test.go`, `windowcounter/bench_yaegi_test.go` | Yaegi | same fake |
| `tokenbucket/bench_e2e_test.go`, `windowcounter/bench_e2e_test.go` | compiled | live Redis / Dragonfly |

Matrix (sub-benchmark names are stable so README rows map 1:1):

- `tokenbucket.Memory`: `hot-key`, `rotating-10k`, `new-key-full-table` (65536 live sources prefilled with a frozen test clock, each iteration admits a never-seen key, so every iteration evicts), `hot-key-parallel` (`b.RunParallel`).
- `tokenbucket.Redis`: `hot-key`, `rotating-10k`, `hot-key-parallel`.
- `windowcounter` `sync_rate` 0: `hot-key`, `rotating-10k`, `hot-key-parallel`.
- `windowcounter` `sync_rate` 20 ms (`minSyncRate`, the most Redis traffic): `hot-key`, `rotating-10k`, `hot-key-parallel`; the real `time.AfterFunc` flush runs in the background and `Close` runs after the timer stops.
- `BenchmarkFlush`: 1000 dirty keys, calls `flushPending` in-package once per iteration after re-dirtying the keys with the timer off (`sync_rate` long enough that no tick fires). Gives the before/after of D4.
- Yaegi (`interp.New` + `stdlib.Symbols`, no unsafe, GOPATH with non-test sources, probe package with a `Loop(count)` function, one warm-up `Eval`, then one timed `Eval` of `Loop(b.N)`, no `ReportAllocs`): `Memory` hot key, `Redis` hot key, `windowcounter` `sync_rate` 0 hot key, `sync_rate` 20 ms hot key.
- Live: one compiled benchmark per limiter and mode with `b.Run("redis"|"dragonfly")`, addresses from `lookupLiveEngineAddrs`, skip under `testing.Short()` or both addresses unset, keys prefixed with a per-run unique prefix. No Yaegi live, no live parallel claims beyond what the run measures.
- Every fake-backed benchmark reports `b.ReportMetric(cmds/float64(b.N), "redis-cmds/op")` from the fake's command counter; compiled ones also `b.ReportAllocs()`.

Alternatives rejected in explore: measuring the flush inside the timed Take loop (flush runs on another goroutine, unstable attribution); Yaegi live (interpretation multiplier already visible against the fake).

### D2. Fake Redis: opt-in digest cache and command counter (test-only)

Each package's `testFakeRedis` gains `scriptCache bool` plus `loadedScripts map[string]string` (same shape as `simpleredis/fake_redis_test.go`), and a `commands int` counter incremented once per served command. With `scriptCache` off (default) EVALSHA still answers NOSCRIPT, so every existing test and every `lastEvalCommand()` assertion is unchanged. Benchmarks turn it on; EVAL stores `sha1hex(body)` and EVALSHA of a known digest runs the stored body. `startTestFakeRedis`, `writeGopath*`, `copyNonTestGo`, `callerDir`, `writeGopathFile` widen to `testing.TB` (all callers pass `*testing.T`; explore compiled this with no caller change).

Rejected: cache always on (empties `lastEvalCommand` assertions), a separate benchmark-only fake (second RESP parser), `SCRIPT LOAD` from the benchmark (no such SimpleRedis verb; API out of scope).

### D3. `tokenbucket.Memory`: intrusive expiry-ordered list (explore option C)

- `memEntry` gains `key string` and `prev, next *memEntry`; `Memory` gains `head, tail *memEntry`. Concrete pointers only — no `container/list`, no `any` (Yaegi).
- Every `Allow` that keeps or creates an entry sets `expireAt = now + ttl` and moves the entry to the tail (O(1)). Because `ttl` is fixed per limiter and `now` is read under `m.mu`, tail-append keeps the list ordered by `expireAt` whenever the clock does not move backwards. Production `time.Now` carries a monotonic reading, and `Add`/`Before` use it, so wall-clock jumps do not break the order.
- New source: pop from the head while the head has expired (removes from map and list); if `len(buckets)` is still at 65536, pop the head (the least recently used live source); then create the full bucket and append it. A present-but-expired entry found on lookup is unlinked and deleted as today.
- `dropExpired` and `dropOne` disappear; `unlinkLocked`, `pushBackLocked`, `removeLocked` (names indicative) are their replacement. The 5 test sites that read `limiter.buckets[...]` keep working; `allow_wait_duration_overflow_test.go`, which inserts a `memEntry` by hand, inserts through the same link helper.
- Spec pins cap, expired-first, and full-bucket restart (`std_go_tokenbucket_allow` ADDED requirement). It deliberately does **not** promise LRU: today's victim is arbitrary, and leaving it unspecified keeps the list an implementation detail.
- Clock moving backwards (`SetNowForTest` only): an entry touched at an earlier `now` is appended behind entries that expire later, so the head may not be the first to expire. Decisions do not change (lookup still checks `expireAt`), the cap still holds, and an expired entry still goes on lookup or when it reaches the head. Accepted; the cap test uses a forward-moving clock.

Rejected (explore table): sweep only at the cap (still O(n) with a full table of live sources — the measured worst case), amortized sweep (needs a full walk to know whether an expired entry exists), `container/list` (`Element.Value` is `any`).

### D4. `windowcounter`: batched flush EVAL (explore option A)

- `flushScript` is replaced by one batch script (a single dirty key is a batch of one; one script, one digest). Lua 5.1-safe, numeric `for` over `#KEYS`, no `unpack` / `table.maxn`, every key in `KEYS` (Dragonfly), following `simpleredis` `msetexFallbackScript`:

  ```lua
  local out = {}
  for i = 1, #KEYS do
    local exists = redis.pcall("exists", KEYS[i])
    local value = redis.pcall("incrby", KEYS[i], ARGV[2 * i - 1])
    if type(value) == "table" then
      out[i] = "err:" .. tostring(value.err)
    else
      if exists == 0 then
        redis.pcall("expireat", KEYS[i], ARGV[2 * i])
      end
      out[i] = value
    end
  end
  return out
  ```

  ARGV holds `delta, expireAt` pairs in `KEYS` order. The reply is a flat array: an integer per applied key, a bulk string `err:<message>` per failed key — never a nested table, which `simpleredis.Eval` would reject.
- **K = 256** keys per EVAL (spec ceiling 1024, the `MSetEX` precedent). At 256 keys a request is roughly 15–20 KB and the script does about 768 cheap calls, which keeps the engine's single-threaded script time short while cutting round trips by 256×. Implement may change K only inside 64–1024 and only with `BenchmarkFlush` evidence recorded on the explore row; the spec does not pin 256.
- Go side, under the same lock discipline as today: snapshot pending windows under `l.mu` (set `flushDelta`), split into batches of K, and for each batch drop `l.mu`, `Eval`, re-lock, then apply per key:
  - integer slot → `redisKnown = n`, `unread = false`, `localDelta -= sent`, `flushDelta = 0`, clear `lastOutageErr` (as today).
  - `err:` slot → `flushDelta = 0`, keep `localDelta`, remember the first error (`errors.New` of the engine message).
  - other slot → same as failed, error wraps the `strconv` failure with `redis:issue?` (keeps "Flush EVAL integer parse keeps the cause").
  - whole-EVAL error, or slot count ≠ batch size → every key of that batch `flushDelta = 0`; first error remembered.
  - After all batches: drop idle expired windows and store the first error in `lastOutageErr` / `lastOutageAt` exactly as today. Take/Peek do **not** return that error (spec "Failed buffered flush is retained").
- Batches run in series, so a later batch's keys stay in flight (`flushDelta` set) until their EVAL returns; a concurrent flusher skips them, as today. Take on any key can run between batches and during each EVAL.
- `EXPIREAT` after an applied `INCRBY` is not checked: once the increment is applied the key must count as applied, or a retry would double count. Today such a failure fails the whole EVAL and the retry double-counts. `EXPIREAT` on an existing key with an integer argument does not fail in practice (it is not a deny-OOM command), so this only removes a theoretical double count.
- Test-only: the `windowcounter` fake learns the batch script (parse `numkeys`, KEYS, ARGV pairs, return the array; record `EXPIREAT` for new keys as today) and gains a per-key failure injection (fail INCRBY for keys with a given prefix, N times) for the new partial-batch tests. The single-key branch matching `args[1] == flushScript` is replaced, not duplicated.

Rejected (explore table): pipeline in SimpleRedis (new public capability, changes `std_go_simpleredis_resp-commands`), concurrent per-key flush workers (competes with the request-path pool, more interpreted goroutines, same command count). A batch without `pcall` is rejected: a mid-script error leaves earlier keys applied while Go retries all of them.

### D5. Before/after measurement and commit order

1. Commit 1: D1 + D2 only (benchmarks, fakes, helper widening), no production change. Run `go test -run '^$' -bench . -benchtime 1s -count 5 ./tokenbucket/ ./windowcounter/` on the reference machine (Intel Core Ultra 7 265K, Windows 11, `go1.25.6 windows/amd64`), keep the raw output, take the median.
2. Commit 2: D3 + D4 with their tests. Same command, same machine.
3. README publishes the commit-2 medians; the `new-key-full-table` and `BenchmarkFlush` rows add a "before (commit `<sha of commit 1>`)" column. The PR card carries both series with both SHAs.

### D6. README and devdocs wording

- README intro keeps "It does not compile your plugin into Traefik" (it describes how Traefik loads plugins) plus a clause that the same packages are also compiled into a custom Traefik build. `## Yaegi` is rewritten: the code runs compiled and under Yaegi, and the Yaegi constraints stay because the interpreted path exists.
- New comparison section (algorithm, where state lives, Redis round trips per request, pros/cons, scalability across instances / key count / Redis outage, benchmark table with command, machine, Go version, `go 1.21` floor note, fake = warm script cache, Windows loopback dominates fake numbers, parallel-vs-fake is bounded by the serialized fake). Traefik Proxy `RateLimit` and Hub `DistributedRateLimit` only from `knowledge/research/ext_traefik_ratelimiter_token-bucket/` and `ext_traefik_hub_distributed-rate-limit/`; `tokenbucket/lua.go` is "derived from Traefik's Lua with two declared changes"; undocumented Hub behavior is "not documented". The batched flush's cluster limitation (CROSSSLOT; SimpleRedis has no cluster support anyway) is stated.
- `std_go_simpleredis.md`: the clause "production is compiled into Traefik, and Yaegi is a portability and e2e-cost concern, not the speed target" becomes "the code runs compiled and under Yaegi"; the encoder rule itself ("do not justify an encoder shape with the interpreted benches") stays, because `std_go_simpleredis_resp-commands` still mandates it. That spec's rationale paragraph still says the deployment is compiled into Traefik; aligning it is a separate spec edit outside Desired 5, recorded as a `note large` follow-up (`devstate/.../issues.md`).
- Benchmark usage line in `std_go_tokenbucket.md`, `std_go_windowcounter.md`, `std_go_test-suites.md`; source cap and batched flush gotchas in the two package packets.

### D7. Spec hosting

FindSpecHost verdicts (recorded in `devstate/.../specs.md`): `std_go_tokenbucket_allow` fold (one ADDED requirement), `std_go_windowcounter_sync-flush` fold (two MODIFIED, one ADDED). Benchmarks, fakes, README, and devdocs carry no live promise and get no spec.

## Risks / Trade-offs

- [A reply lost after the engine applied a batch re-applies up to K keys on retry; today the window is one key] → Same accepted EVAL double-apply risk as `std_go_simpleredis.md` already states; K bounded at 256; documented in README/devdoc.
- [One EVAL with 256 keys blocks a single-threaded Redis longer than one key] → About 768 O(1) calls; microseconds, far below the 900 ms command timeout. `BenchmarkFlush` and live benchmarks show it.
- [Dragonfly multi-key script behavior with `redis.pcall` is inferred, not measured] → New live scenario in `e2e-dragonfly` (spec "Several keys in one EVAL on both engines"); `msetexFallbackScript` already runs multi-key there.
- [Cluster engines answer CROSSSLOT for a multi-key EVAL] → Not supported today either (no `MOVED` handling); README and devdoc say so.
- [Eviction victim changes from arbitrary to least recently used] → Not specified before or after; no test pins it; spec keeps it unspecified.
- [Backward test clock breaks list order] → Decisions unchanged, cap unchanged; documented in D3.
- [README numbers are pinned to one machine and date; CI does not run benchmarks] → Table states machine, Go version, command, and date; reproducible with `git checkout`.

## Migration Plan

1. Commit 1 (benchmarks + fakes), measure baseline.
2. Commit 2 (D3, D4, new tests), `go test -short ./tokenbucket/ ./windowcounter/`, Yaegi tests, then live Go E2E Redis and Dragonfly in CI.
3. README and devdocs with final numbers.
4. Rollback: revert commit 2; no persisted state changes shape (Redis keys and values are the same; only how many keys travel per EVAL).
