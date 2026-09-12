# SimpleRedis

## Language

**SimpleRedis**:
A stdlib pooled TCP RESP client (`New(Config)`, `Get`, `MGet`, `Set` with EX, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX`, `MSetEXAt`, `ExecPipeline`, `Close`). `New` copies `Config` and does not dial; the first command dials. Pool, timeout, and retry knobs live on `Config` and freeze at `New`.
_Avoid_: `go-redis`, miniredis, TLS, Unix sockets, renaming the package to `redis`

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis`. Callers match errors by `Error()` text (`redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`). The copied sources are Apache-2.0 (`LICENSE`).

## How to use

- Call `simpleredis.New(simpleredis.Config{Host: host})` once before concurrent use. Set pool, timeout, and retry knobs on `Config` (`-1` turns extra retries or backoff off). After `New` those knobs do not change.
- Do not dial in Traefik `New`. Call `simpleredis.New` there; first command in `ServeHTTP` after Redis is up (`Set`, `Get`, `Incr`, `Eval`, `MSetEX`, or `ExecPipeline`).
- Call `MSetEX(names, values, seconds)` or `MSetEXAt(names, values, unixSeconds)` for many keys with one TTL. Do not MSET then EXPIRE. Match integer `0` as `redis:issue?`.
- Match AUTH-class Redis errors as `redis:noauth`. Do not type-assert `net.Error` (Yaegi).
- For a mixed batch, pass argv rows to `ExecPipeline`. Empty or nil `commands` returns nil, nil and does not dial. More than 64 commands returns `redis:issue?` and does not send. Inspect each slot’s `Values` and `Err` by `Error()` text; the batch error is only I/O, protocol, cap, unreachable, or timeout. Do not use `ExecPipeline` for a same-TTL group write; that is `MSetEX`.
- Prove with `go test ./simpleredis/...` (includes Yaegi GOPATH interp). Traefik e2e is `./Test-Integration.ps1` (Redis and Dragonfly).
- Call `Eval(script, keys, args)` with the Lua body. Eval hashes the body each call (cheap SHA-1; no map, no lock) and sends EVALSHA; on NOSCRIPT it falls back once to EVAL so the engine stores the script. Do not SCRIPT LOAD at `New`.

## Pattern snippet

```go
client := simpleredis.New(simpleredis.Config{Host: "redis:6379"})
if err := client.Set("k", []byte("v"), 60); err != nil {
	return err
}
got, err := client.Get("k")
if err != nil {
	return err
}
n, err := client.Incr("counter")
if err != nil {
	return err
}
if err := client.MSetEX([]string{"a", "b"}, [][]byte{[]byte("1"), []byte("2")}, 60); err != nil {
	return err
}
slots, err := client.ExecPipeline([][][]byte{
	{[]byte("INCR"), []byte("c")},
	{[]byte("EXPIRE"), []byte("c"), []byte("60")},
})
if err != nil {
	return err
}
_ = slots
```

## Key files

- `simpleredis/simpleredis.go` — client, New, Close
- `simpleredis/config.go` — freeze-at-New knobs
- `simpleredis/pool.go` — in-use turns, unused sockets, dial
- `simpleredis/commands.go` — thin verbs
- `simpleredis/commands_exec.go` — exec, retry
- `simpleredis/commands_eval.go` — Eval (EVALSHA, NOSCRIPT → EVAL)
- `simpleredis/commands_msetex.go` — MSetEX / MSetEXAt, capability cache
- `simpleredis/commands_pipeline.go` — ExecPipeline, PipelineSlot
- `simpleredis/resp.go` — RESP codec
- `simpleredis/yaegi_test.go` — interpreter New/Get/Set/Del/Incr/Eval/MSetEX/ExecPipeline
- `e2e/simpleredisprobe/plugin.go` — Traefik local plugin
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`
- `knowledge/devdocs/std_go_simpleredis_resp-decode.md` — ReadSlice decode, escaping `+`/`:` copies, `parseLen`
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md`

## Gotchas

- `New` does not dial. A refusing host is fine until the first command.
- After `Close`, later commands return `redis:unreachable` and do not redial. Closed-client unreachable is not retried. Call `Close` while a command is in flight: that command may finish; its socket is closed on release, not returned to idle.
- Live cap is `Config.PoolSize` (`PoolSize()`, const default 8; unused plus in-flight). `New` builds the in-use-turn channel. After `New` a `PoolSize` write does not resize. A waiter past `PoolSize` waits `PoolTimeout` (default 200ms), then `redis:unreachable`. Idle trim is `MaxIdleConns` (default 8). Pool wait is not retried.
- Yaegi tests copy non-test sources into GOPATH with stdlib only (`useunsafe` false).
- `Incr` / `IncrBy` do not refresh TTL. `Expire` / `ExpireAt` integer `0` is success, not `redis:miss`. `MSetEX` / `MSetEXAt` integer `0` (and any integer other than `1`) is `redis:issue?`.
- Eval scripts that touch keys must list those keys in `keys` (Dragonfly rejects undeclared keys). Do not use `table.maxn` (Dragonfly Lua 5.4). Same KEYS rule applies to every EVAL inside `ExecPipeline`.
- `MSetEX` / `MSetEXAt` reject empty or mismatched slices and more than 1024 pairs with `redis:issue?` before dial. Zero or negative TTL is passed through, same as `Set`. Clustered engines need every key in one hash slot (hash tags); the client does not hash-tag or split.
- Redis 7 and Dragonfly have no native `MSETEX`. The first call sends native, then caches Lua `Eval` (names in KEYS, values then `EX`/`EXAT` then TTL in ARGV). Redis 8.4+ / Valkey 9.1+ stay on native after a successful `MSETEX`. A later `ERR unknown command` recaches Lua and `Eval`s that call. Past `MSetEXAt` may return no error while a later `Get` is `redis:miss`.
- Eval hashes the Lua body on every call and sends EVALSHA. SHA-1 of a limiter script (~470 B) is cheaper than a mutex on the Traefik hot path, and a `map[string]string` of full script bodies would need a lock because Go maps are not concurrent. Do not cache digests. SCRIPT FLUSH or a restart yields NOSCRIPT; Eval then sends EVAL once so the engine stores the script. Callers still pass the body.
- Every verb uses go-redis-shaped command retry (`MaxRetries` / `MinRetryBackoff` / `MaxRetryBackoff` on `Config`; `0` is default 3 extra retries / 8ms / 512ms; `-1` is off). Retry `redis:unreachable` (including a fresh dial) and LOADING/READONLY/MASTERDOWN/CLUSTERDOWN/TRYAGAIN / max-clients replies. Do not retry `redis:timeout` or a pool-wait timeout. INCR/INCRBY/EVAL can double-apply after a lost reply; that is accepted.
- `ExecPipeline` caps at 64 commands. A `-` reply on one slot does not fail the batch error and does not stop later reads. After a successful Flush, do not retry the whole batch (double-apply). `ExecPipeline` does not rewrite EVAL rows to EVALSHA; that remains `Eval`.
- RESP2 null array `*-1` is `redis:issue?` (this client has no BLPOP/MULTI/EXEC). Null bulk `$-1` is `redis:miss`. Do not treat them as the same.
- A server-closed idle socket younger than 30s is borrowed and retried as `redis:unreachable` (`io.EOF`). Do not prove peer close by closing the client fd (`SetDeadline` then fails with `os.ErrClosed` and never reaches `ioError`).
