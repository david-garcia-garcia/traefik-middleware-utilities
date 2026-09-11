## Why

SimpleRedis can `MGet` many keys in one round trip but can only write one key with a TTL (`Set` → `SET k v EX n`). Limiter flush paths need many keys, one shared expiry, atomically. Redis 7 and Dragonfly v1.40.2 have no native `MSETEX`; Valkey 9.1 and Redis 8.4 do. A Lua `EVAL` fallback is the default path those engines will take.

## What Changes

- Add Yaegi-safe `MSetEX(names []string, values [][]byte, seconds int64) error` and `MSetEXAt(..., unixSeconds int64) error` on `SimpleRedis`. Parallel slices, reject empty or mismatched input, cap at 1024 pairs, always send `EX` or `EXAT` (no NX/XX/PX/KEEPTTL on the Go API).
- Native path: `MSETEX <numkeys> k1 v1 … EX|EXAT <n>`. Integer `1` is success; integer `0` is `redis:issue?` (not swallowed). Detect native once per client by sending the real command; cache under the existing mutex; `ERR unknown command` → Lua for that call and later calls.
- Fallback: one Lua 5.1-safe `EVAL` script (`for i = 1, #KEYS`, no `unpack` / `table.unpack` / `table.maxn`), keys declared in KEYS (Dragonfly), values then token then TTL in ARGV. Reuse `Eval`. No EVALSHA.
- Tests: fake native argv; fake unknown-command then cached EVAL; live Redis 7 and Dragonfly Lua with TTL landed; Yaegi both paths; Pester `/redis` and `/dragonfly` headers for the new verb. Native argv stays on the fake (no Valkey/Redis 8 compose service).

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: add `MSetEX` / `MSetEXAt` (native `MSETEX` plus Lua fallback, capability cache, pair cap); prove both paths under Yaegi; live Redis 7 and Dragonfly MUST show TTL landed; Traefik e2e `/redis` and `/dragonfly` MUST exercise the new verb.

## Impact

- `simpleredis/simpleredis.go`, `simpleredis_test.go`, `yaegi_test.go`, new `simpleredis/live_test.go`.
- `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1`, CI `test` job env `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`. Compose services stay `redis:7-alpine` and `dragonfly:v1.40.2`.
- Main spec `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` in implement / devdocsimpact (API unshipped in explore).
- Out of scope: NX/XX/PX/KEEPTTL on the Go API; EVALSHA; pipeline; MULTI/EXEC; limiter flush callers; `go-redis` / miniredis; changing `MGet` empty-input.
