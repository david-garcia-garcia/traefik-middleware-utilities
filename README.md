# traefik-middleware-utilities

Shared Go libraries for Traefik middlewares.

Traefik loads local and catalog plugins through [Yaegi](https://github.com/traefik/yaegi), an interpreter. This repo exists so the pieces those middlewares keep rewriting — Redis, a reclaim table, and rate-limit primitives — live in one place and stay inside the subset of Go that Yaegi can run.

## Libraries

Each package is one job. Import the one that matches the middleware; do not mix their clocks.

| Library | Status | Package | Role |
| --- | --- | --- | --- |
| Reclaim table | Current | `reclaim/` | In-process table: one value per key, with create / sleep / wake / close when Traefik reloads config. |
| SimpleRedis | Current | `simpleredis/` | Stdlib RESP client (GET/MGET/SET/DEL/INCR/EXPIRE/EVAL/MSetEX). Middlewares construct this with `New(Config)` instead of inventing a Redis client. Apache-2.0 (copied from crowdsec-bouncer). |
| Window counter | Current | `windowcounter/` | Kong-style **sliding-window hit counter** on Redis or Dragonfly. Counts hits in a time window and returns allow/deny plus a sliding estimate. Local memory is only the `sync_rate` flush buffer. |
| Token bucket | Current | `tokenbucket/` | Traefik **token bucket** (refill + burst). Same math in-process and on Redis/Dragonfly via `Eval`. |

### What each one does

**Reclaim table** (`reclaim/`) stores one Go value per key for the life of a Traefik plugin instance. Use it when the middleware must keep a client, a counter, or a limiter across requests and tear it down on reload (`Sleep` / `Wake` / `Close`). It does not talk to Redis and it does not decide admit/deny.

**SimpleRedis** (`simpleredis/`) is the shared Redis/Dragonfly session. `New(Config)` stores host, password, and database; the first command dials. Use it for GET/SET/INCR/EVAL/MSetEX. It does not implement a rate-limit algorithm.

**Window counter** (`windowcounter/`) is a distributed **hit counter**: `Take(key, limit, window)` increments the current window and admits while `current + previous × (1 − elapsed/window)` is still at or under `limit`. Redis (or Dragonfly) holds the integers. `sync_rate=0` talks to Redis on every Take; `sync_rate>0` buffers locally and flushes with EVAL. Pass its `Sleep`/`Wake`/`Close` into reclaim when the counter is the stored value.

**Token bucket** (`tokenbucket/`) is Traefik RateLimit’s clock: `Allow(key)` refills at `rate`, caps at `burst`, and returns allowed plus wait. In-process uses a mutex map; Redis uses `Eval` of the copied Lua (`#rl_source == 4`). Do not mix this clock with `windowcounter/`.

This is **not** Traefik’s HTTP RateLimit middleware. Token bucket and window counter are separate packages; pick the clock the middleware actually uses.

## Yaegi

This code is interpreted inside Traefik, not compiled into it. Treat that as a hard constraint, not a later port.

Rules of thumb:

- Prefer the Go standard library. Every import is also interpreted.
- No cgo, no `unsafe`, no assembly, no `//go:embed` unless Yaegi in the target Traefik version is known to support it.
- Avoid features the Traefik Yaegi build does not implement (some generics, some `reflect`, runtime-heavy packages).
- Keep APIs simple: concrete types, explicit methods, no clever init-time magic.
- Prove each package with tests that a normal `go test` run covers, then sanity-check the same sources under Yaegi before calling a library done.

If a change would be fine in compiled Go but fails under Yaegi, the Yaegi failure wins.

## Layout

```text
reclaim/         reclaim table
simpleredis/     stdlib RESP client (Apache-2.0)
windowcounter/   sliding-window Redis/Dragonfly hit counter
tokenbucket/     Traefik token bucket (in-process and Redis EVAL)
e2e/             fake Traefik plugins + Pester harness (Yaegi)
```

Module path: `github.com/david-garcia-garcia/traefik-middleware-utilities`.

## Tests

Four suites (see `knowledge/devdocs/std_go_test-suites.md`):

```text
go test -short ./...          # unit Go — fake TCP, no Redis
go test ./...                # also Go E2E when *_LIVE_REDIS and *_LIVE_DRAGONFLY are set
./Test-Integration.ps1      # Pester — Traefik local plugins
```

`Test-Integration.ps1` starts Traefik v3.7.11 with fake local plugins (`e2e/reclaimprobe`, `e2e/simpleredisprobe`) so reclaim and SimpleRedis run under Yaegi. Docker is required. The copied SimpleRedis client is Apache-2.0 (`LICENSE`).

Go E2E files are `{domain}_e2e_test.go` next to that domain (`commands_e2e_test.go`, `limiter_e2e_test.go`). They skip under `-short` or when both live addrs are unset, and fail if exactly one addr is set. Set both `SIMPLEREDIS_LIVE_*`, `WINDOWCOUNTER_LIVE_*`, and `TOKENBUCKET_LIVE_*` (Redis `:6379`, Dragonfly `:6380`). Passworded AUTH proof uses `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` (`:6381` / `:6382`).

CI (`.github/workflows/ci.yml`) runs golangci-lint, unit `go test -short` (no engines), Go E2E (`go test` with Redis 7 and Dragonfly), and that same Pester harness on every pull request and on pushes to `master`.

Tag a version (`v1.0.0`) to cut a GitHub release via GoReleaser (source archive + SBOM; no plugin binary).
