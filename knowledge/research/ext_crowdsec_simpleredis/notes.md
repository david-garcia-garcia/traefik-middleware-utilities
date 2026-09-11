# SimpleRedis package

Pinned clone: `https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin` @ `6548da47e933efe60309954be5f764f839b69e3f` (shallow `master`, 2026-09-11). Package on disk: `pkg/simpleredis/` (`simpleredis.go` + `simpleredis_test.go` only).

## Module and import path

Module path is `github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin` (`go.mod`). Import path of the package is `github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/pkg/simpleredis` (`pkg/cache/cache.go` import). The GitHub fork URL is `david-garcia-garcia/crowdsec-bouncer-traefik-plugin`; the Go module path still uses `maxlerebourg`.

## Exported API

There is no `New` constructor. Callers allocate `&SimpleRedis{}` (or a value) and call `Init`.

From `pkg/simpleredis/simpleredis.go`:

| Kind | Name | Role |
|---|---|---|
| const string | `RedisUnreachable` `"redis:unreachable"` | Dial / closed-client / I/O mapped to unreachable |
| const string | `RedisMiss` `"redis:miss"` | Null bulk (`$-1`) |
| const string | `RedisTimeout` `"redis:timeout"` | I/O deadline (`os.ErrDeadlineExceeded`) |
| const string | `RedisNoAuth` `"redis:noauth"` | AUTH-class error prefixes |
| const string | `RedisIssue` `"redis:issue?"` | Malformed / unexpected reply shape |
| type | `SimpleRedis` | Host, password, database, idle pool, `closed` |
| method | `Init(host, pass, database string)` | Sets fields; comment: call once before concurrent use; not mutex-protected |
| method | `Get(name string) ([]byte, error)` | `GET` |
| method | `MGet(names []string) ([][]byte, error)` | `MGET`; empty/nil names return `nil, nil` with no dial |
| method | `Set(name string, data []byte, duration int64) error` | `SET` + `EX` + decimal duration |
| method | `Del(name string) error` | `DEL` |
| method | `Close()` | Drains idle pool, sets `closed`; further Get/Set/Del/MGet return unreachable and do not dial; idempotent |

Unexported: `pooledConn`, `exec`, `borrow`, `release`, `dial`, `do`, `writeCommand`, `readReply`, `readBulk`, `readLine`, `replyError`, `ioError`. Pool caps: `maxIdleConns = 8`, `idleTimeout = 30s`, `dialTimeout = 2s`, `ioTimeout = 1s`.

Extract: [.sources/simpleredis.go.md](.sources/simpleredis.go.md)

## Redis wire commands and data model

RESP arrays only on the write path (`*` + `$` bulk args), TCP `net.Dialer.Dial("tcp", host)` — no Unix socket, no TLS, no `go-redis` (`simpleredis.go` `dial` / `writeCommand`; `go.mod` has no Redis client require).

Commands sent:

- Per new dial, if `pass != ""`: `AUTH <pass>`
- Per new dial, if `database != ""`: `SELECT <database>`
- `GET <key>`
- `MGET <keys...>`
- `SET <key> <value> EX <duration>` (duration always sent as `EX`; integer seconds via `strconv.FormatInt`)
- `DEL <key>`

Keys and values are opaque byte strings. Null bulk is a miss (`errMiss`). Array replies for `MGET` keep a `nil` slot when that key is a null bulk. Status (`+`) and integer (`:`) replies are treated as success payloads. Error replies (`-`) with prefixes `NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH` become `redis:noauth`; other `-` text is returned as `errors.New(text)`.

Idle connections older than 30s are closed on borrow; a dead pooled conn is retried once unless the error is timeout (`exec`). After `Close`, `borrow` returns `redis:unreachable` without dialing.

Extract: [.sources/simpleredis.go.md](.sources/simpleredis.go.md)

Cache-layer key shape (not SimpleRedis itself): `pkg/cache` prefixes keys as `prefix + ":" + key` when prefix is non-empty (`prefixed`). LAPI passes `CachePrefix(config)`: session hex in stream/alone, identity hex in live/none (`pkg/lapi/session.go`). Decision payloads stored through cache are `"t"` / `"f"` / `"c"` (`pkg/decisionscope/lookup.go`). Range membership uses cache key `"range-index"` (`pkg/decisionscope/scope.go`).

Extracts: [.sources/cache.go.md](.sources/cache.go.md), [.sources/client.go.md](.sources/client.go.md), [.sources/session.go.md](.sources/session.go.md), [.sources/lookup.go.md](.sources/lookup.go.md), [.sources/scope.go.md](.sources/scope.go.md)

## Tests

Only test file: `pkg/simpleredis/simpleredis_test.go`. No miniredis, no httptest, no live Redis process. Helpers:

- `startFakeRedis`: in-process TCP listener + `map[string]string` store; understands AUTH/SELECT/GET/MGET/SET
- `startStaticRedis`: same listener shape, every command gets a canned RESP reply

Coverage (test names): Get hit/miss; sequential reuse (25 Gets → 1 conn); concurrent pool cap (8 goroutines, ≤8 conns); values with newlines; MGet hits/misses/empty/newline alignment/short reply → `redis:issue?`; `-NOAUTH` → `redis:noauth`; SET error reply passthrough; Del against `+OK`; unreachable `127.0.0.1:1`; stale pooled conn retry; Close drains idle and blocks redial; AUTH+SELECT once per dial not per GET; timeout on reused conn is not retried; I/O timeout on first dial; idle-timeout eviction.

Extract: [.sources/simpleredis_test.go.md](.sources/simpleredis_test.go.md)

`pkg/cache/cache_test.go` constructs `*simpleredis.SimpleRedis` for round-robin pointer identity and `Client.New(..., true, ...)` Close/GetMany-unreachable; it does not speak Redis protocol except via the real client against `127.0.0.1:1` (connection refused). E2e mock `tests/e2e/mock/mocklapi/main.go` `serveRedis` speaks RESP arrays “as spoken by pkg/simpleredis” plus inline GET; it is not a `simpleredis` unit test.

Extracts: [.sources/cache_test.go.md](.sources/cache_test.go.md), [.sources/mocklapi-main.go.md](.sources/mocklapi-main.go.md)

## Dependencies and Yaegi-sensitive surface

`pkg/simpleredis/simpleredis.go` imports only stdlib: `bufio`, `errors`, `io`, `net`, `os`, `strconv`, `strings`, `sync`, `time`. No `unsafe`, no `import "C"` / cgo, no type parameters. `go.mod` `require` is only `github.com/leprosus/golang-ttl-map v1.1.7` (used by `pkg/cache` memory path, not by simpleredis). `go.sum` lists that module only; no `github.com/redis/*`, `github.com/go-redis/*`, `github.com/alicebob/miniredis`, or `github.com/maxlerebourg/simpleredis`.

`ioError` uses `errors.Is(err, os.ErrDeadlineExceeded)` and comments that a `net.Error` assert has panicked across the Yaegi interpreter boundary. That is a source comment about a past Yaegi panic, not a measured Traefik run in this clone.

Extracts: [.sources/simpleredis.go.md](.sources/simpleredis.go.md), [.sources/go.mod.md](.sources/go.mod.md)

## How the bouncer uses it

`plugin.go` does not import `pkg/simpleredis`. The only production importer is `pkg/cache`.

`cache.redisCache` holds `writer *simpleredis.SimpleRedis` and `readers []*simpleredis.SimpleRedis`. `Client.New(..., isRedis true, writeHost, readHosts, pass, database, keyPrefix)` allocates a distinct pointer per host, `Init`s it, and must not copy the struct after Init (comment: pool mutex). Reads: `Get` / `MGet` on `nextReader()` (round-robin over readers, or writer if none). Writes: `Set` / `Del` on `writer`. `Close` calls `SimpleRedis.Close` on writer and each reader. `RedisMiss` / `RedisUnreachable` are remapped to `cache:miss` / `cache:unreachable` via `err.Error()` string match.

`pkg/lapi.Client.New` constructs `cache.Client` with `config.RedisCacheEnabled`, `RedisCacheHost`, `RedisCacheReadHosts`, `RedisCachePassword`, `RedisCacheDatabase`, and `CachePrefix(config)`. `lapi.Client.Close` calls `cacheClient.Close`. `lapi.Prepare` resolves `RedisCachePassword` via `GetVariable`. Config defaults: Redis off; host `"redis:6379"`; empty read hosts, password, database; `RedisCacheUnreachableBlock: true` (`pkg/configuration/configuration.go`). Captcha and decisionscope talk to `*cache.Client`, not to SimpleRedis directly.

Extracts: [.sources/cache.go.md](.sources/cache.go.md), [.sources/client.go.md](.sources/client.go.md), [.sources/session.go.md](.sources/session.go.md), [.sources/configuration.go.md](.sources/configuration.go.md), [.sources/plugin.go.md](.sources/plugin.go.md)

## License

Repo root `LICENSE` is Apache License 2.0. Appendix copyright lines: `Copyright 2020 Containous SAS` and `Copyright 2020 Traefik Labs`. `pkg/simpleredis/` has no package-local LICENSE file.

Extract: [.sources/LICENSE.md](.sources/LICENSE.md)
