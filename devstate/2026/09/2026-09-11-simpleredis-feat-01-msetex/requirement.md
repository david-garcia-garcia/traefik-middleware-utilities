# Requirement
IssueKey: 2026-09-11-simpleredis-feat-01-msetex

## Problem
SimpleRedis can `MGet` many keys in one round trip but can only write one key with a TTL (`Set` → `SET k v EX n`). Limiter flush paths need many keys, one shared expiry, atomically. Redis and Dragonfly have no native `MSETEX`; Valkey does. A Lua fallback is the default path.

## Current (code)
- `simpleredis/simpleredis.go` — exported `Get`, `MGet`, `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`; no `MSetEX` / `MSetEXAt`; no `MSETEX` argv; no cached engine-capability flag on `SimpleRedis`.
- `simpleredis/simpleredis.go` `MGet` — empty/nil names return `nil, nil` and do not dial (not an error).
- `simpleredis/simpleredis.go` `Set` — one name, one value, `EX` seconds; no multi-key write.
- `simpleredis/simpleredis.go` `Eval` — sends `EVAL` + script body every call; no `EVALSHA` / `SCRIPT LOAD`; no `NOSCRIPT` handling.
- `simpleredis/simpleredis.go` `exec` — one command per round trip; no `MULTI`/`EXEC`; no pipeline.
- `simpleredis/simpleredis.go` `mu` — guards pool/closed only.
- `simpleredis/simpleredis_test.go` `startFakeRedis` — AUTH, SELECT, GET, MGET, SET, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL; unknown verbs (including `MSETEX`) reply `+OK`.
- `simpleredis/yaegi_test.go` — Init/Get/Set/Del/Incr/Eval only.
- `e2e/simpleredisprobe/plugin.go` — probe headers for Get/MGet/Del/Incr/IncrBy/Expire/ExpireAt/Eval; no group-write verb.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — GET/MGET/SET/DEL/INCR/EXPIRE/EVAL; EVALSHA MUST NOT be added; no MSETEX.
- `knowledge/devdocs/std_go_simpleredis.md` — same verb list; no group write.
- `knowledge/research/index_ext_redis.md` — INCR, EXPIRE, EVAL only; no MSETEX/MSET finding.
- `knowledge/research/index_ext_dragonfly.md` — EVAL and container image; no MSETEX.

## Desired
- Add Yaegi-safe `MSetEX(names []string, values [][]byte, seconds int64) error` and `MSetEXAt(..., unixSeconds int64) error`: parallel slices, `len(names) == len(values)`, reject empty input, cap pair count so one call cannot build an unbounded frame, require the TTL argument (do not omit expiration; do not default KEEPTTL).
- Native path: `MSETEX <numkeys> k1 v1 … EX|EXAT <n>`. Integer `1` is success; integer `0` must be returned to the caller (not swallowed). No NX/XX on this API.
- Fallback: one `EVAL` script, Lua 5.1- and 5.4-safe (no `unpack`/`table.unpack`), loop `SET … EX` / `EXAT`. Past `EXAT` may delete keys and still succeed (`1`); do not hide that.
- Detect native support once per client, cache under the existing mutex; treat `ERR unknown command` as not supported; do not probe native on every call; cached miss goes straight to EVAL.
- Document that clustered engines need all keys in one hash slot (hash tags).
- Fake-server tests for native argv, unknown-command fallback then cached EVAL, mismatched lengths before I/O, past EXAT. Equivalence of both paths. Yaegi coverage for both paths. Live e2e on Redis and Dragonfly asserting TTL landed (Valkey native when available).

## Affected
- `simpleredis/simpleredis.go`, `simpleredis/simpleredis_test.go`, `simpleredis/yaegi_test.go`
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` (change folder in propose/implement)
- `knowledge/devdocs/std_go_simpleredis.md`
- `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1`

## Out of scope
- NX/XX, PX/PXAT, KEEPTTL on the Go API.
- EVALSHA / SCRIPT LOAD / NOSCRIPT plumbing (sibling perf-05); pipeline (perf-04); MULTI/EXEC; pool/timeout changes.
- Implementing limiter flush callers (`handoff-leaky-bucket`, `handoff-traefik-token-limiter`).
- `go-redis`, miniredis, TLS, Unix sockets.
- Changing `MGet` empty-input behavior.

## Unknowns
- Pair-count cap value (ticket says cap; no number).
- How integer `0` is spelled as a Go error (`errors.New` of the decimal, `redis:issue?`, or another stable string).
- Whether empty slices should error or match dest `MGet` (`nil, nil`).
- Official `MSETEX` argv/reply and Redis/Dragonfly non-support are ticket claims; no `knowledge/research/` finding for Valkey `MSETEX` (indexes cover INCR/EXPIRE/EVAL only). Re-detect-after-reconnect policy (“only if cheap”).

## Tensions
- Finding cites `MGet` `:101-103` as “reject empty”; dest `MGet` returns `nil, nil` and does not dial. Desired still says reject empty for `MSetEX`; do not change `MGet`.
- Finding says build fallback plumbing together with perf-05 `EVALSHA`; dest spec `std_go_simpleredis_resp-commands` forbids EVALSHA. This ticket is feat-01 only; EVALSHA stays out.
- Finding line numbers for `MGet`/`Set` predate INCR/EVAL already on dest (`326b2f1`).
- Finding API is `error`; “surface `0`” is not a returned integer — explore must pick the error spelling.
