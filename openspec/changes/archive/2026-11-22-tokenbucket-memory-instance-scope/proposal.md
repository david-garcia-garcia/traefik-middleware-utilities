# Proposal

## Why

Traefik issue [#13706](https://github.com/traefik/traefik/issues/13706) (open PR [#13823](https://github.com/traefik/traefik/pull/13823), closed PR [#13707](https://github.com/traefik/traefik/pull/13707)) reports that N routers referencing one RateLimit middleware get N in-memory budgets per source while Redis keys by middleware and shares one. This library already matches that split: each `*Memory` is its own map; Redis `Allow` evals an opaque caller key on the server. The catalog and GoDoc do not yet name that scope, so a caller can treat the Traefik mismatch as a library bug. The existence of the defect was taken from those upstream reports; this change does not rediscover it.

## What Changes

- No limiter behavior change. `Memory.Allow`, `Redis.Allow`, `NewMemory`, and `NewRedis` stay as they are.
- GoDoc on `Memory` / `NewMemory`: each value is its own key space; to share a budget across routers or reloads, share one `*Memory` (for example `reclaim` keyed by middleware name).
- GoDoc on `Redis` / `NewRedis`: state lives on the server; any limiter on the same key shares it; the key prefix defines the scope.
- Contract tests (compiled `(a)`/`(b)`, live `(c)`) that assert that documented scope, with helpers suffixed `13706`. Control tests `TestLive_Limiter` / `twoInstancesShare` and `TestRedis_TwoInstancesShareBurst` stay.
- Usage gotcha on `knowledge/devdocs/std_go_tokenbucket.md`: two `*Memory` values admit `2*burst`; share one value for one budget. Language and README stay.
- Middleware-level fix (one limiter per middleware name, Traefik `#13823` `scope`) stays a note: `knowledge/debt/2026-11-22-tokenbucket-consumer-limiter-scope.md`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: ADDED live promise that each `*Memory` is its own key space (two values on the same opaque `key` admit `2*burst`) and that any `*Redis` on the same `key` shares server state (admit `burst` in total). Prefixing of `key` remains the caller's job. Do not touch `std_go_tokenbucket_lua-eval` or `std_go_tokenbucket_status`.

## Impact

- GoDoc: `tokenbucket/memory.go`, `tokenbucket/redis.go`.
- Tests: `tokenbucket/traefik_issue13706_test.go` (new), `tokenbucket/traefik_issue13706_e2e_test.go` (new). Reuse `runForEachLiveEngine` / `waitLiveClient` in `tokenbucket/limiter_e2e_test.go`; do not move them.
- Spec delta: `openspec/changes/tokenbucket-memory-instance-scope/specs/std_go_tokenbucket_allow/spec.md`.
- Usage: `knowledge/devdocs/std_go_tokenbucket.md` gotcha only.
- Debt already on disk: `knowledge/debt/2026-11-22-tokenbucket-consumer-limiter-scope.md`, `devstate/issues.md`.
- Unchanged: `reclaim/`, `simpleredis/`, `windowcounter/`, `backendbackoff/`, README, Lua script, constructors, `Allow` signatures.
