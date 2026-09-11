# SimpleRedis

## Language

**SimpleRedis**:
A stdlib pooled TCP RESP client (`Init`, `Get`, `MGet`, `Set` with EX, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `Close`). `Init` stores host, password, and database and does not dial; the first command dials.
_Avoid_: `go-redis`, miniredis, TLS, Unix sockets, renaming the package to `redis`

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis`. Callers match errors by `Error()` text (`redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`). The copied sources are Apache-2.0 (`simpleredis/LICENSE`).

## How to use

- Allocate `&simpleredis.SimpleRedis{}` and `Init(host, pass, database)` once before concurrent use.
- Do not dial in Traefik `New`. Call `Init` there; first command in `ServeHTTP` after Redis is up (`Set`, `Get`, `Incr`, or `Eval`).
- Match AUTH-class Redis errors as `redis:noauth`. Do not type-assert `net.Error` (Yaegi).
- Prove with `go test ./simpleredis/...` (includes Yaegi GOPATH interp). Traefik e2e is `./Test-Integration.ps1` (Redis and Dragonfly).

## Pattern snippet

```go
client := &simpleredis.SimpleRedis{}
client.Init("redis:6379", "", "")
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
```

## Key files

- `simpleredis/simpleredis.go` — session, pool, RESP
- `simpleredis/yaegi_test.go` — interpreter Init/Get/Set/Del/Incr/Eval
- `e2e/simpleredisprobe/plugin.go` — Traefik local plugin
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`
- `knowledge/devdocs/std_go_simpleredis_resp-decode.md` — ReadSlice decode, escaping `+`/`:` copies, `parseLen`
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md`

## Gotchas

- `Init` does not dial. A refusing host is fine until the first command.
- After `Close`, commands return `redis:unreachable` and do not redial.
- Idle pool cap is eight after release; concurrent in-flight dials are not capped (copied client).
- Yaegi tests copy non-test sources into GOPATH with stdlib only (`useunsafe` false).
- `Incr` / `IncrBy` do not refresh TTL. `Expire` / `ExpireAt` integer `0` is success, not `redis:miss`.
- Eval scripts that touch keys must list those keys in `keys` (Dragonfly rejects undeclared keys). Do not use `table.maxn` (Dragonfly Lua 5.4).
