# Proposal

## Why

Traefik confirms a race on concurrent first requests from a new source ([traefik/traefik#14328](https://github.com/traefik/traefik/issues/14328), first mentioned in [traefik/traefik#13957](https://github.com/traefik/traefik/issues/13957)): `ttlmap` Get and Set take the lock separately, each goroutine creates a full bucket, and the last write wins. This library already holds one mutex across Memory lookup/create/consume and admits Redis through one atomic Eval. Explore measured the defect does not reproduce here. The change is regression tests that keep that true, plus spec scenarios that name the concurrent-first-request promise.

## What Changes

- No product change. Do not edit `tokenbucket/memory.go`, `tokenbucket/redis.go`, or `tokenbucket/lua.go`.
- Add `tokenbucket/traefik_issue14328_test.go` (`TestTraefik14328_MemoryConcurrentFirstRequests`: 20 rounds) and `tokenbucket/traefik_issue14328_e2e_test.go` (`TestTraefik14328_RedisConcurrentFirstRequests`: 10 rounds against live Redis and Dragonfly).
- Load: 16 goroutines on a shared barrier, 1 req/min, burst 5, maxDelay 500 ms, ttl 120 s. Count `Status.Allowed`. Assert admits ≤ burst. An `Allow` error fails the test.
- Files are self-contained for this issue. Helpers in those files only, suffix `14328`. Do not copy the upstream-repro worktree tests (they mix other issues and unpack a three-value `Allow`).
- Live gating matches `TestLive_Limiter` via `runForEachLiveEngine`. The live client uses PoolSize 16; readiness wait is local `waitLiveClient14328`. Do not change shared `waitLiveClient`.
- The PR summary states the upstream defect does not occur here and why (one mutex on Memory; atomic Eval on Redis) and keeps both upstream issue links.
- Spec deltas fold one concurrent-first-request scenario into `std_go_tokenbucket_allow` (Memory mutex span, admits ≤ burst) and one into `std_go_tokenbucket_lua-eval` (atomic Eval, same load). No new spec folder.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: 16 simultaneous first Allows on a new source SHALL admit at most `burst`; the in-process store holds the bucket-map mutex across lookup, create, and consume.
- `std_go_tokenbucket_lua-eval`: the same concurrent first-request load on a new Redis/Dragonfly key SHALL admit at most `burst` because one Eval owns HGETALL/HSET/EXPIRE; Go MUST NOT GET or SET that hash.

## Impact

- **Code**: none. `Memory.Allow`, `Redis.Allow`, and `allowScript` stay as dest.
- **Tests**: new `tokenbucket/traefik_issue14328_test.go` and `tokenbucket/traefik_issue14328_e2e_test.go`. Reuse `runForEachLiveEngine` from `tokenbucket/limiter_e2e_test.go`. Do not change `waitLiveClient`. No Yaegi file for this issue. No shared helper file.
- **Specs**: deltas under `openspec/changes/tokenbucket-concurrent-first-requests/specs/std_go_tokenbucket_allow/` and `std_go_tokenbucket_lua-eval/`.
- **Callers / APIs**: none. Dest `Allow` remains `(Status, error)`.
- **Unchanged**: `simpleredis` (except PoolSize/MaxIdleConns/PoolTimeout on the issue's live client), other Traefik issue tests, `dropExpired` / C2, Traefik itself.
