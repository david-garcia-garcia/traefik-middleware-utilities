# SimpleRedis

## Language

**SimpleRedis**:
A stdlib pooled TCP RESP client (`New(Config)`, `Get`, `MGet`, `Set` with EX, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX`, `MSetEXAt`, `Close`). `New` copies `Config` and does not dial; the first command dials. Pool, timeout, and retry knobs live on `Config` and freeze at `New`.
_Avoid_: `go-redis`, miniredis, TLS, Unix sockets, renaming the package to `redis`

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis`. Callers match errors by `Error()` text (`redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`). The copied sources are Apache-2.0 (`LICENSE`).

## How to use

- Call `simpleredis.New(simpleredis.Config{Host: host})` once before concurrent use. Set pool, timeout, and retry knobs on `Config` (`-1` turns extra retries or backoff off). After `New` those knobs do not change. Set `Pass` and `Database` on `Config` when AUTH or SELECT is needed.
- Do not dial in Traefik `New`. Call `simpleredis.New` there; first command in `ServeHTTP` after Redis is up (`Set`, `Get`, `Incr`, `Eval`, or `MSetEX`).
- Call `MSetEX(names, values, seconds)` or `MSetEXAt(names, values, unixSeconds)` for many keys with one TTL. Do not MSET then EXPIRE. Match integer `0` as `redis:issue?`.
- Match AUTH-class Redis errors as `redis:noauth`. Do not type-assert `net.Error` (Yaegi).
- Prove with `go test ./simpleredis/...` (includes Yaegi GOPATH interp). Handshake AUTH/SELECT failure is the in-process fake plus skip-if-unset live engines: `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` (pool wait and SELECT 99) and `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` (WRONGPASS). Traefik e2e is `./Test-Integration.ps1` (Redis and Dragonfly, including handshake failure routes).
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
- `simpleredis/live_test.go` — skip-if-unset pool wait, SELECT 99, WRONGPASS, and MSetEX TTL
- `simpleredis/yaegi_test.go` — interpreter New/Get/Set/Del/Incr/Eval/MSetEX
- `e2e/simpleredisprobe/plugin.go` — Traefik local plugin (`Host`, optional `Password`, `Database`, `DropHost`)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`

## Gotchas

- `New` does not dial. A refusing host is fine until the first command.
- After `Close`, commands return `redis:unreachable` and do not redial. Closed-client unreachable is not retried.
- Live cap is `Config.PoolSize` (`PoolSize()`, const default 8; unused plus in-flight). `New` builds the in-use-turn channel. After `New` a `PoolSize` write does not resize. A waiter past `PoolSize` waits `PoolTimeout` (default 200ms), then `redis:unreachable`. Idle trim is `MaxIdleConns` (default 8). Pool wait is not retried.
- Yaegi tests copy non-test sources into GOPATH with stdlib only (`useunsafe` false).
- `Incr` / `IncrBy` do not refresh TTL. `Expire` / `ExpireAt` integer `0` is success, not `redis:miss`. `MSetEX` / `MSetEXAt` integer `0` (and any integer other than `1`) is `redis:issue?`.
- Eval scripts that touch keys must list those keys in `keys` (Dragonfly rejects undeclared keys). Do not use `table.maxn` (Dragonfly Lua 5.4).
- `MSetEX` / `MSetEXAt` reject empty or mismatched slices and more than 1024 pairs with `redis:issue?` before dial. Zero or negative TTL is passed through, same as `Set`. Clustered engines need every key in one hash slot (hash tags); the client does not hash-tag or split.
- Redis 7 and Dragonfly have no native `MSETEX`. The first call sends native, then caches Lua `Eval` (names in KEYS, values then `EX`/`EXAT` then TTL in ARGV). Redis 8.4+ / Valkey 9.1+ stay on native after a successful `MSETEX`. A later `ERR unknown command` recaches Lua and `Eval`s that call. Past `MSetEXAt` may return no error while a later `Get` is `redis:miss`.
- Eval hashes the Lua body on every call and sends EVALSHA. SHA-1 of a limiter script (~470 B) is cheaper than a mutex on the Traefik hot path, and a `map[string]string` of full script bodies would need a lock because Go maps are not concurrent. Do not cache digests. SCRIPT FLUSH or a restart yields NOSCRIPT; Eval then sends EVAL once so the engine stores the script. Callers still pass the body.
- Every verb uses go-redis-shaped command retry (`MaxRetries` / `MinRetryBackoff` / `MaxRetryBackoff` on `Config`; `0` is default 3 extra retries / 8ms / 512ms; `-1` is off). Retry `redis:unreachable` (including a fresh dial) and LOADING/READONLY/MASTERDOWN/CLUSTERDOWN/TRYAGAIN / max-clients replies. Do not retry `redis:timeout`, a pool-wait timeout, `redis:noauth`, or other Redis `-ERR` replies including handshake AUTH/SELECT failures. INCR/INCRBY/EVAL can double-apply after a lost reply; that is accepted.
- Match AUTH-class prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) as `redis:noauth`. Redis 7.4 `AUTH` against a nopass default user returns `ERR AUTH <password> called without any password configured…`, which is **not** that prefix — it is a plain error. Dragonfly without `--requirepass` accepts any `AUTH` password (`OK`). Wrong-password live proof needs `requirepass` on both engines.
- Empty password skips AUTH; empty database skips SELECT. When both are set, AUTH runs before SELECT.
- `SELECT` out of range is `ERR DB index is out of range`, not `redis:noauth`. A handshake AUTH or SELECT error closes the socket, is not pooled, and is not retried.
- Probe success labels on `/redis` and `/dragonfly` stay host-only (empty password and database). Failure routes `/redis-wrong-password` `/dragonfly-wrong-password` set `Password`; `/redis-database-99` `/dragonfly-database-99` set `Database`.
