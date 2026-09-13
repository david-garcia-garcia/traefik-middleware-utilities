# SimpleRedis

## Language

**SimpleRedis**:
A stdlib pooled TCP RESP client (`New(Config)`, `Get`, `MGet`, `Set` with EX, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX`, `MSetEXAt`, `Close`). Every verb takes `context.Context` as its first argument. `New` copies `Config` and does not dial; the first command dials. Pool, timeout, and retry knobs live on `Config` and freeze at `New`. A caller with no deadline passes `context.Background()`.
_Avoid_: `go-redis`, miniredis, TLS, Unix sockets, renaming the package to `redis`

**Reply boundary**:
The point after one complete RESP value where the connection reader has no leftover bytes. A socket that is not on a reply boundary is destroyed, not returned to idle.
_Avoid_: draining leftover bytes to resynchronise; treating a clean parse as reusable without an empty reader

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis`. Callers match errors with `errors.Is` against the exported sentinels (`ErrMiss`, `ErrUnreachable`, `ErrPoolWait`, `ErrUnsupportedReply`, …) or `IsMiss` / `IsUnreachable` / `IsPoolWait`. The string consts (`redis:miss`, `redis:unreachable`, `redis:unsupported-reply`, …) stay for display and legacy text matching. The copied sources are Apache-2.0 (`LICENSE`).

## How to use

- Call `simpleredis.New(simpleredis.Config{Host: host})` once before concurrent use. Set pool, timeout, and retry knobs on `Config` (`MaxRetries` 0 at New means 1 extra retry; `-1` turns extra retries or backoff off). After `New` those knobs do not change. Set `Pass` and `Database` on `Config` when AUTH or SELECT is needed. One hop’s worst-case wait is `(MaxRetries+1)*(DialTimeout+IOTimeout)` (600ms at zero Config). Eval NOSCRIPT (EVALSHA then EVAL) and MSetEX unknown-command (native then Eval) are separate hops, each with that budget; do not treat one public verb as one stacked deadline. Every verb takes a context first (`Get(ctx, name)`). Pass `req.Context()` on the request path; pass `context.Background()` when there is no deadline.
- Do not dial in Traefik `New`. Call `simpleredis.New` there; first command in `ServeHTTP` after Redis is up (`Set`, `Get`, `Incr`, `Eval`, or `MSetEX`).
- Call `MSetEX(ctx, names, values, seconds)` or `MSetEXAt(ctx, names, values, unixSeconds)` for many keys with one TTL. Do not MSET then EXPIRE. Match integer `0` as `redis:issue?`.
- Match AUTH-class Redis errors as `redis:noauth` (`ErrNoAuth`). Match miss with `IsMiss`, unreachable with `IsUnreachable`, pool saturation with `IsPoolWait`. Do not type-assert `net.Error` (Yaegi).
- Prove with `go test -short ./simpleredis/...` (unit + Yaegi fake). Live Redis/Dragonfly is `*_e2e_test.go` (see `knowledge/devdocs/std_go_test-suites.md`): dest engines for pool wait, SELECT 99, and CLIENT KILL; `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` for WRONGPASS. Yaegi live covers every public verb. Traefik e2e is `./Test-Integration.ps1` (`scripts/integration-tests.simpleredis.Tests.ps1`): one file; `-Engine redis` or `-Engine dragonfly` sets `INTEGRATION_ENGINE` and drives `/<engine>/<verb>` (status + body). Exact `/redis` and `/dragonfly` are health Set+Get. Helpers live in `scripts/integration-tests.utils/`. CI splits reclaim (`Integration Tests`) from those two engine jobs. Allocation guards are `TestAlloc*` functions that call `testing.Benchmark` with `ReportAllocs` and fail on over-budget allocs/op or B/op; they do not need `-bench`. They skip when the race detector is on.
- Call `Eval(ctx, script, digest, keys, args)` with the Lua body and `ScriptSHA1Hex(script)` (compute once at init for a reused script). Eval MUST NOT hash. It sends EVALSHA of that digest; on NOSCRIPT it falls back once to EVAL so the engine stores the script. Do not SCRIPT LOAD at `New`. A digest that does not match the body pays EVAL every call. Return values are a flat array of bulk strings or integers; wrap each Lua slot with `tostring` (or return numbers). Top-level Lua `false`/`nil` is one nil slot with a nil error, not `redis:miss`. Nested tables and `{err=...}` inside an array are `redis:unsupported-reply` and close the socket. Traefik `/eval` takes that digest as query `digest`.

## Pattern snippet

```go
client := simpleredis.New(simpleredis.Config{Host: "redis:6379"})
ctx := context.Background()
if err := client.Set(ctx, "k", []byte("v"), 60); err != nil {
	return err
}
got, err := client.Get(ctx, "k")
if err != nil {
	return err
}
n, err := client.Incr(ctx, "counter")
if err != nil {
	return err
}
if err := client.MSetEX(ctx, []string{"a", "b"}, [][]byte{[]byte("1"), []byte("2")}, 60); err != nil {
	return err
}
```

## Key files

- `simpleredis/simpleredis.go` — client, New, Close
- `simpleredis/config.go` — freeze-at-New knobs
- `simpleredis/pool.go` — in-use turns, unused sockets, dial
- `simpleredis/commands.go` — thin verbs
- `simpleredis/commands_exec.go` — exec, retry
- `simpleredis/commands_eval.go` — Eval (EVALSHA, NOSCRIPT → EVAL)
- `simpleredis/commands_msetex.go` — MSetEX / MSetEXAt, capability cache
- `simpleredis/resp.go` — RESP codec
- `simpleredis/pool_e2e_test.go` — live pool wait, SELECT 99, WRONGPASS, CLIENT KILL recover
- `simpleredis/commands_e2e_test.go`, `commands_eval_e2e_test.go`, `commands_msetex_e2e_test.go` — live verbs
- `simpleredis/simpleredis_e2e_test.go` — skip/fail harness
- `simpleredis/yaegi_e2e_test.go` — interpreted live verbs
- `simpleredis/bench_test.go` — encode/decode benches and CI alloc guards
- `simpleredis/interpretedcost_test.go` — asserting Yaegi conversion matrix, copy-vs-unsafe benches, and Yaegi encode cost
- `simpleredis/yaegi_test.go` — interpreter New/Get/Set/Del/Incr/Eval/MSetEX; live LiveVerbs is the full public set
- `e2e/simpleredisprobe/plugin.go` — Traefik local plugin (`Host`, optional `Password`, `Database`, `DropHost`)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`
- `knowledge/devdocs/std_go_simpleredis_resp-decode.md` — ReadSlice decode, escaping `+`/`:` copies, `parseLen`
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md`

## Gotchas

- `New` does not dial. A refusing host is fine until the first command.
- After `Close`, later commands return `redis:unreachable` and do not redial. Closed-client unreachable is not retried. Call `Close` while a command is in flight: that command may finish; its socket is closed on release, not returned to idle.
- Live cap is `Config.PoolSize` (`PoolSize()`, const default 8; unused plus in-flight). `New` builds the in-use-turn channel. After `New` a `PoolSize` write does not resize. A waiter past `PoolSize` waits `PoolTimeout` (default 200ms), then `redis:unreachable`. Idle trim is `MaxIdleConns` (default 8). Pool wait is not retried. An extra return of an in-use turn when the channel is already full does not hang: it is dropped and `OverFrees()` increments. Dest used to park that goroutine forever. Correct accounting stays at `OverFrees() == 0`.
- Borrow sweeps idle sockets older than `IdleTimeout` (default 30s) and closes them, including a stale head behind a younger tail that is reused. `New` starts no reaper; a fully quiet client keeps those sockets until `Close`.
- A client that did not come from `New` (`&SimpleRedis{}`) fails the first command immediately with `redis:unreachable` and is not retried.
- Yaegi tests copy non-test sources into GOPATH with stdlib only (`useunsafe` false).
- Do not adopt `unsafe` zero-copy between `string` and `[]byte`. Compiled it saves a little on Eval encode; under Yaegi (the Traefik plugin path) the legacy struct-header trick is a net loss versus inline `[]byte(s)`. Keep `[]byte(...)` / `string(...)` in session source.
- Do not import `unsafe` in session source. Do not set `useUnsafe` on the plugin manifest or compose `settings.useunsafe`. Traefik registers unsafe symbols only when **both** the manifest and operator settings are true; manifest true and settings false refuses to load the plugin (`knowledge/research/ext_traefik_plugins_useunsafe/`).
- CI fails if the Yaegi conversion matrix cells change, if a non-test session file imports `unsafe` or `"C"`, or if probe/compose `useUnsafe` becomes true. Named copy-vs-unsafe benches reproduce ns/op (`go test -run XXX -bench`); they do not fail `go test` without `-bench`.
- Yaegi v0.16.1 does not export `unsafe.Slice` / `unsafe.String` / `StringData` / `SliceData` even when unsafe symbols are registered (`knowledge/research/ext_traefik_plugins_yaegi-unsafe/`).
- `Incr` / `IncrBy` do not refresh TTL. `Expire` / `ExpireAt` integer `0` is success, not `redis:miss`. `MSetEX` / `MSetEXAt` integer `0` (and any integer other than `1`) is `redis:issue?`.
- Eval scripts that touch keys must list those keys in `keys` (Dragonfly rejects undeclared keys). Do not use `table.maxn` (Dragonfly Lua 5.4).
- `MSetEX` / `MSetEXAt` reject empty or mismatched slices and more than 1024 pairs with `redis:issue?` before dial. Zero or negative TTL is passed through, same as `Set`. Clustered engines need every key in one hash slot (hash tags); the client does not hash-tag or split.
- Redis 7 and Dragonfly have no native `MSETEX`. The first call sends native, then caches Lua `Eval` (names in KEYS, values then `EX`/`EXAT` then TTL in ARGV). Redis 8.4+ / Valkey 9.1+ stay on native after a successful `MSETEX`. A later `ERR unknown command` recaches Lua and `Eval`s that call. Native MSETEX and that Eval are separate hops; Eval starts with a full command budget. Past `MSetEXAt` may return no error while a later `Get` is `redis:miss`.
- Eval sends EVALSHA of the caller digest. Compute `ScriptSHA1Hex` once next to a reused script const. Do not keep a client digest table. SCRIPT FLUSH or a restart yields NOSCRIPT; Eval then sends EVAL once so the engine stores the script. A wrong digest repeats that fallback. Callers still pass the body for that miss path. EVALSHA and EVAL are separate hops; each binds its own overall deadline so EVAL still has a full command budget. Do not share remaining time across hops (that can starve EVAL).
- Every verb uses go-redis-shaped command retry (`MaxRetries` / `MinRetryBackoff` / `MaxRetryBackoff` on `Config`; `MaxRetries` 0 at New means 1 extra retry; backoff `0` is 8ms / 512ms; `-1` is off). Each hop (`exec`) has an overall deadline `(maxRetries+1)*(DialTimeout+IOTimeout)` (600ms at zero Config: 200ms dial, 100ms I/O, two attempts). Eval NOSCRIPT and MSetEX unknown-command are two or three hops; each hop binds a fresh budget. When that instant is sooner than the caller, `exec` binds it with `context.WithDeadline` (same as `net.Dialer`). Remaining time is shared by dial, AUTH, SELECT, and that hop’s command; each socket op still `SetDeadline`s `IOTimeout` or whatever is left. Retry `redis:unreachable` (including a fresh dial) and LOADING/READONLY/MASTERDOWN/CLUSTERDOWN/TRYAGAIN / max-clients replies. Do not retry `redis:timeout`, a pool-wait timeout, a client that did not come from `New`, a cancelled context, `redis:noauth`, or other Redis `-ERR` replies including handshake AUTH/SELECT failures. INCR/INCRBY/EVAL can double-apply after a lost reply; that is accepted. `ErrPoolWait` and a client-not-from-New both wrap `ErrUnreachable`, so `IsUnreachable` is true; use `IsPoolWait` to shed load. Do not rewrite retry onto `errors.Is(err, ErrUnreachable)` alone.
- RESP2 null array `*-1` is `redis:issue?` (this client has no BLPOP/MULTI/EXEC). Null bulk `$-1` is a nil slot from decode. Get maps a one-slot nil reply to `redis:miss`. Eval `return false` / `return nil` is that same wire and MUST NOT be `redis:miss`. Empty bulk `$0` is a non-nil empty slice, not a miss. Do not treat null array and null bulk as the same.
- A well-framed type this decoder does not decode (nested array, `-` inside an array, HTTP-shaped or RESP3 type byte) is `redis:unsupported-reply`, not `redis:issue?`. The socket is discarded. Do not retry it.
- After one complete reply, leftover unread RESP means the peer is ahead of the protocol. That command still returns its decoded value; the socket is discarded, not drained, and not pooled. Do not drain leftover bytes to resynchronise. A compliant peer leaves the reader empty, so sequential Gets still reuse one connection. Unread leftover before the next write on the same socket (AUTH then SELECT) is `redis:issue?` and is not written.
- A server-closed idle socket younger than 30s is borrowed and retried as `redis:unreachable` (`io.EOF`). Do not prove peer close by closing the client fd (`SetDeadline` then fails with `os.ErrClosed` and never reaches `ioError`).
- A server-closed idle socket younger than 30s is borrowed and retried as `redis:unreachable` (`io.EOF`). Do not prove peer close by closing the client fd (`SetDeadline` then fails with `os.ErrClosed` and never reaches `ioError`).
- Match AUTH-class prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) as `redis:noauth`. Redis 7.4 `AUTH` against a nopass default user returns `ERR AUTH <password> called without any password configured…`, which is **not** that prefix — it is a plain error. Dragonfly without `--requirepass` accepts any `AUTH` password (`OK`). Wrong-password live proof needs `requirepass` on both engines.
- Empty password skips AUTH; empty database skips SELECT. When both are set, AUTH runs before SELECT.
- `SELECT` out of range is `ERR DB index is out of range`, not `redis:noauth`. A handshake AUTH or SELECT error closes the socket, is not pooled, and is not retried.
- Probe success labels on `/redis` and `/dragonfly` stay host-only (empty password and database). Failure routes `/redis-wrong-password` `/dragonfly-wrong-password` set `Password`; `/redis-database-99` `/dragonfly-database-99` set `Database`.
- Probe verb paths (`/<engine>/<verb>`) are terminal: HTTP 200 + Redis body or 502 + `err.Error()`. They do not forward to next. Set `IOTimeout` to 1s on the probe clients so a 500ms TIME-wait Eval survives the 100ms default.
