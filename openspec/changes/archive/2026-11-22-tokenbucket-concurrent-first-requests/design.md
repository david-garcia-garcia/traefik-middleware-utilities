# Design

## Context

See `proposal.md` — Why. Explore measured dest does not reproduce the Traefik Get/Set race: `Memory.Allow` holds `m.mu` from `memory.go:71` through lookup, TTL drop, `makeRoomLocked`/`storeLocked`, `consumeOne`, and `expireAt`; `Redis.Allow` is one `Eval` of `allowScript` (`redis.go:67`). Probe: Memory 20×16 max admits 5; Redis 16379 and Dragonfly 16380 10×16 PoolSize 16 max admits 5.

Constraints that shape the tests:

- Dest `Allow` is `(Status, error)`. Count `status.Allowed`. Do not unpack three values (the read-only repro worktree still does).
- `runForEachLiveEngine` (`limiter_e2e_test.go:74`) is the skip/table gate. `waitLiveClient` builds `simpleredis.New(Config{Host: addr})` (PoolSize 8) and has four call sites in `tokenbucket/`. PoolSize 16 cannot go through that helper.
- Reference tests at `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros` are read-only. They mix other issues and must not be copied into this tree.
- Identity owner is the caller (`std_go_tokenbucket_allow` “Caller owns the key”). Tests pass an opaque string. Do not derive a client address, user, tenant, Host, or trust hop.

## Goals / Non-Goals

**Goals:**

- Regression tests that 16 simultaneous first requests from one new source never admit more than burst (1 req/min, burst 5, maxDelay 500 ms). Deterministic and fast.
- Memory: 20 rounds. Live Redis and Dragonfly: 10 rounds, PoolSize 16. Goroutines start on a shared barrier.
- PR summary names both upstream issues and states the defect does not occur here (one mutex; atomic Eval).

**Non-Goals:**

- Product change to `Memory.Allow`, `Redis.Allow`, or `allowScript`.
- Changing shared `waitLiveClient`.
- Shared helper files, copying other issues' tests, a Yaegi file, TTL/control cases the 14328 reference does not have, C2 `dropExpired` measurement, fixing Traefik.

## Decisions

### D1. Tests only; if a later run admits more than burst, stop

No product fix in this change. Explore already owns the measurement (not reproduced). If `go test` later sees admits > burst, the run is `blocked`. Do not invent a product patch.

Rejected: patching Memory/Redis “just in case” — Out of scope; dest already serializes lookup/create/consume.

### D2. Two self-contained files, helpers suffixed `14328`

- `tokenbucket/traefik_issue14328_test.go` — `TestTraefik14328_MemoryConcurrentFirstRequests`
- `tokenbucket/traefik_issue14328_e2e_test.go` — `TestTraefik14328_RedisConcurrentFirstRequests`

Helpers live in those files: `concurrentFirstAdmits14328`, `liveKey14328`, `waitLiveClient14328`. Each is `t.Helper()` (thelper). Do not extract into `limiter_e2e_test.go`. Do not copy `traefik_issues_test.go` from the repro worktree.

Opaque keys: Memory uses `"192.0.2.1"`; Redis uses `liveKey14328(t, "source-"+strconv.Itoa(round))` so rounds do not share a hash. The library MUST NOT rebuild that key.

Rejected: one shared `traefik_issues_test.go` — mixes #13957/#13704 and the old three-value `Allow`.

### D3. Shared barrier, then count `Status.Allowed`

`concurrentFirstAdmits14328`: `ready` WaitGroup, `start` chan, `done` WaitGroup. Each goroutine `ready.Done()`, `<-start`, then `Allow(context.Background(), key)`. After `ready.Wait()`, `close(start)`. Count `status.Allowed`. First non-nil error `t.Fatal`. Assert `got <= burst`. No real-time sleep on the Memory path.

Load: `NewMemory` / `NewRedis` at `1.0/60`, burst `5`, `500*time.Millisecond`, `120*time.Second`. New Memory limiter per round so the source is new. Redis: new key per round, one limiter.

Rejected: TTL control subtest — the 14328 reference has none; that is #13957.

### D4. Live gate via `runForEachLiveEngine`; local `waitLiveClient14328` with PoolSize 16

Call `runForEachLiveEngine(t, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY", ...)` like `TestLive_Limiter`. Skip/`-short`/one-engine stay that helper's job.

Client: `simpleredis.New(Config{Host: addr, PoolSize: 16, MaxIdleConns: 16, PoolTimeout: 2 * time.Second})`. Readiness: local `waitLiveClient14328` retries Eval `"return 1"` up to 15s (same wait loop as `waitLiveClient`, different Config). Do not change `waitLiveClient`. Deviation already on `devstate/deviations.md`.

Rejected: adding PoolSize to `waitLiveClient` — shared helper, four call sites, Out of scope.

### D5. Spec fold, no new leaf

Deltas fold into `std_go_tokenbucket_allow` and `std_go_tokenbucket_lua-eval`. No `std_go_tokenbucket_status` delta (`Allowed` mapping unchanged). No Yaegi file (`std_go_test-suites` documents Yaegi for Allow; this ticket does not name one).

## Risks / Trade-offs

- [A later run admits more than burst] → Stop (`blocked`). Do not patch product in this change.
- [PoolSize 8 would serialize 16 first requests] → D4 PoolSize 16, MaxIdleConns 16, PoolTimeout 2s so idle trim and 200 ms default timeout cannot turn overlap into `redis:unreachable`.
- [Flaky overlap] → 20 Memory rounds and 10 live rounds; barrier starts all goroutines together; no extra sleep.
- [Copying the repro worktree compiles against dest] → D2: dest `(Status, error)` only; do not copy those files.

## Migration Plan

Tests only. Rollback is reverting the commit. No Redis key format change. Mixed versions sharing a bucket keep today's meaning.

## Open Questions

None. Explore closed control cases, `Allow` signature, identity owner, file names, and helper placement. PoolSize vs `waitLiveClient` is the taken deviation.
