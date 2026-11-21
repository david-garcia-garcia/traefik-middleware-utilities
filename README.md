# traefik-middleware-utilities

[![Build Status](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/workflows/ci.yml/badge.svg)](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/david-garcia-garcia/traefik-middleware-utilities)](https://goreportcard.com/report/github.com/david-garcia-garcia/traefik-middleware-utilities)
[![Latest GitHub release](https://img.shields.io/github/v/release/david-garcia-garcia/traefik-middleware-utilities?sort=semver)](https://github.com/david-garcia-garcia/traefik-middleware-utilities/releases/latest)
[![License](https://img.shields.io/badge/license-Apache%202.0-brightgreen.svg)](LICENSE)

Shared Go libraries for Traefik middlewares.

## Why this exists

Traefik loads local and catalog plugins through [Yaegi](https://github.com/traefik/yaegi), an interpreter. It does not compile your plugin into Traefik.

That is why this repo exists. The official Go Redis client (and other compiled-only libraries) will not run under Yaegi. A middleware that imports them cannot be loaded the way Traefik actually loads plugins, and cannot be tested that way either.

These packages are the pieces those middlewares otherwise rewrite: a Redis client, a reclaim table, two rate-limit clocks, an in-memory backend backoff gate, and a CIDR lookup helper. They stay in the subset of Go that Yaegi can interpret, and they are tested under Yaegi, not only as compiled Go. The same packages are also compiled when a middleware is built into a custom Traefik binary, so they run both ways.

They look like unrelated libraries. They share one module on purpose: the window counter and the token bucket are built on SimpleRedis, reclaim is how a middleware keeps a client or gate across a Traefik reload, and not every package talks to Redis. Splitting them would hide that they are one middleware stack under the same Yaegi constraint.

## Reclaim table

Package `reclaim/`. Keep one Go value per key for the life of a Traefik plugin instance — a Redis client, a counter, a limiter.

Call `table.Open` from the plugin constructor (`New`), on a `*Table` the plugin package holds (`reclaim.New`). Pass Traefik's constructor context, not `req.Context()`. The first `Open` for a key runs `create` once. Later `Open`s on the same key return that same value. When Traefik reloads config it cancels the old context; the table sleeps the value, and if a new `New` opens the same key before grace ends, it wakes the stored value instead of creating another.

Pass `Sleep` / `Wake` / `Close` as `reclaim.Hooks` when the stored value has those methods. Prefix keys when more than one type shares a table.

```go
var table = reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})

var limiter *windowcounter.Limiter
stored, err := table.Open(ctx, "window:"+hash, logger, func() (any, error) {
	var createErr error
	limiter, createErr = windowcounter.New(client, 0)
	return limiter, createErr
}, reclaim.Hooks{
	Sleep: func() { limiter.Sleep() },
	Wake:  func() { limiter.Wake() },
	Close: func() { limiter.Close() },
})
if err != nil {
	return nil, err
}
limiter = stored.(*windowcounter.Limiter)
```

## SimpleRedis

Package `simpleredis/`. A stdlib Redis/Dragonfly client (GET, MGET, SET, DEL, INCR, EXPIRE, EVAL, MSetEX). Use this instead of go-redis: that client does not interpret under Yaegi.

Call `simpleredis.New` in Traefik `New`. It stores host, password, and database; it does not dial. The first command in `ServeHTTP` dials. Pass `req.Context()` on the request path.

```go
client := simpleredis.New(simpleredis.Config{Host: "redis:6379"})
got, err := client.Get(req.Context(), "k")
if err != nil {
	return err
}
```

Set `Pass` and `Database` on `Config` when AUTH or SELECT is needed. Match errors with `simpleredis.IsMiss`, `IsUnreachable`, and `errors.Is` against the exported sentinels — not by comparing error text. Call `Close` when the client is done; later commands do not redial. Apache-2.0 (`LICENSE`).

## Window counter

Package `windowcounter/`. A sliding-window hit counter: how many hits landed on a key in the last window. Kong-style, not Traefik RateLimit.

`Take` records one hit and returns whether it is still under `limit`, plus the sliding estimate. `Peek` is the same observation without recording a hit. The estimate mixes this window with the previous one so the count slides instead of resetting at the boundary.

Build it on a SimpleRedis you already constructed. Prefix keys in the caller. Do not mix this clock with `tokenbucket/`.

```go
counter, err := windowcounter.New(client, 0)
if err != nil {
	return err
}
allowed, estimated, err := counter.Peek(req.Context(), "ip:"+ip, 100, time.Minute)
if err != nil {
	return err
}
if !allowed {
	return errLimited
}
// call backend; on failure:
_, _, err = counter.Take(req.Context(), "ip:"+ip, 100, time.Minute)
```

`sync_rate` `0` talks to Redis on every Take or Peek and returns that call's Redis error. A positive duration batches locally and flushes on that interval; while Redis is down, buffered Take/Peek keep this instance's `limit` with a nil error (per-node cap), not a retained flush error, and retry Redis one `sync_rate` after the last failure. Pass `Sleep` / `Wake` / `Close` into reclaim when the counter is the stored value. Do not close the Redis client from the counter.

## Token bucket

Package `tokenbucket/`. Traefik RateLimit's clock: tokens refill at `rate` per second, cap at `burst`, one token per `Allow`. It is not a hit counter.

`NewMemory` is in-process. `NewRedis` shares the bucket across Traefik instances on a SimpleRedis you already constructed. `Allow` returns whether the request is allowed and how long to wait. The library does not sleep and does not write HTTP 429 — the middleware does.

```go
limiter, err := tokenbucket.NewRedis(client, 100, 50, 5*time.Millisecond, 2*time.Second)
if err != nil {
	return err
}
allowed, wait, err := limiter.Allow(req.Context(), "ip:"+ip)
if err != nil {
	return err
}
if !allowed {
	return errLimited
}
_ = wait
```

Compute `rate` from Traefik `average/period`. Prefix keys in the caller. Two limiter instances on one Redis key share burst. Do not mix this clock with `windowcounter/`.

## Rate limiters compared

Two packages limit request rates: `tokenbucket/` (two stores) and `windowcounter/` (two modes). They are different clocks, not two speeds of one clock. `backendbackoff/` is not a rate limiter: it gates calls into an unhealthy backend.

| | `tokenbucket.NewMemory` | `tokenbucket.NewRedis` | `windowcounter`, `sync_rate` 0 | `windowcounter`, `sync_rate` > 0 |
|---|---|---|---|---|
| Algorithm | Token bucket: refill at `rate`/s, cap at `burst`, one token per `Allow`; Traefik's Lua formulas computed in Go | Same token bucket, in a Lua script derived from Traefik's `AllowTokenBucketRaw` | Sliding window: `current + previous × (1 − elapsed/window)`, whole-second windows | Same sliding window |
| Where state lives | One map per limiter instance in the process, at most 65536 sources, each expiring `ttl` after its last use | One Redis hash per key (`tokens`, `last`), `EXPIRE ttl` | One Redis integer per key and window, TTL two windows | Redis integers as in exact mode, plus a local delta per window key in the process |
| Redis round trips per request | 0 | 1 (`EVALSHA`; one extra `EVAL` when the engine does not have the script yet) | 2 per Take (one `EVAL` of INCR plus EXPIRE-if-no-TTL, one `GET` of the previous window); 2 `GET`s per Peek | Usually 0. A `GET` when a key is first seen, after a flush leaves its local delta at 0, and at most once per `sync_rate` for the previous window. A timer flushes dirty keys every `sync_rate` in ⌈keys/256⌉ `EVAL`s |
| Several Traefik instances | Not shared: each instance has its own buckets, so N instances admit up to N times the rate | Shared: one bucket per key for all instances | Shared and exact: every hit is counted in Redis before the decision | Shared after each flush. Between flushes each instance decides from its own view, so instances together can admit more than `limit` within one `sync_rate` |
| Many keys | `Allow` on a stored key and on a new key is O(1). At 65536 sources a new key first drops expired sources, then one live source, which comes back as a full bucket | Nothing per key in the process; Redis holds one hash per active key | Nothing per key in the process; Redis holds two counters per active key | Memory per active window key. Flush cost grows with dirty keys, 256 keys per `EVAL` |
| Redis outage | Not affected | `Allow` returns the Redis error; the middleware decides what to do | Take and Peek return the Redis error | Take and Peek return a nil error and keep this instance's `limit` (per-node cap). Redis is retried one `sync_rate` after the last failure; unsent deltas stay pending |

Pros and cons:

- `tokenbucket.NewMemory`: no network and the lowest cost per request. The limit is per Traefik instance, and the 65536-source cap evicts a live source under a large key set.
- `tokenbucket.NewRedis`: one shared bucket across instances with one round trip per request. Every request depends on Redis, and an outage is an error the caller has to handle.
- `windowcounter`, `sync_rate` 0: exact shared count. Two round trips per Take, and Redis errors reach the caller. Denied Takes still count.
- `windowcounter`, `sync_rate` > 0: close to in-memory cost per request and keeps working through a Redis outage. The shared limit is approximate between flushes, and outage admits are per node.

The buffered flush sends several keys in one `EVAL`, so all of them have to live on one Redis node. A Redis Cluster engine or cluster proxy would answer `CROSSSLOT`; SimpleRedis does not support Redis Cluster either.

### Compared with Traefik's rate limiters

Traefik Proxy `RateLimit` (open source, read at commit `903e8a9`): a token bucket configured with `average`, `period`, and `burst`. Without Redis it keeps a `golang.org/x/time/rate` limiter per source in an in-memory map capped at 65536 sources. With Redis it runs the Lua script `AllowTokenBucketRaw` (`EVALSHA`, then `EVAL` when the engine does not have the script). The maximum wait is computed from the rate, not configured. The middleware waits up to that delay, answers 429 with `Retry-After` above it, and answers 500 when Redis returns an error; its documentation says nothing about a Redis outage.

`tokenbucket/` follows that clock. `NewRedis` runs a script derived from Traefik's with two declared changes: `#rl_source == 4` replaces `table.maxn` (Dragonfly runs Lua 5.4), and a missing hash starts as a full bucket with `last` at the request time instead of `last` 0. `NewMemory` computes the same formulas in Go instead of using `x/time/rate`. The library does not sleep and does not write HTTP: the middleware passes `maxDelay` and `ttl`, and decides how to answer a denied request or a Redis error.

Traefik Hub `DistributedRateLimit` (commercial; only its documentation was read, on 2026-11-21): documented as a token bucket with `limit`, `period`, and `burst`, kept in Redis so the limit applies across the cluster. Redis is required and can be a single instance, Redis Cluster, or Sentinel. `denyOnError` (default `true`) answers 429 when the limit cannot be read from Redis, and `responseHeaders` adds `X-RateLimit-*` headers. Its internals are unknown: the documentation does not say whether it caches locally, how instances synchronize, or how many Redis round trips a request costs, so no performance comparison is made here. This repo does not reproduce Hub's options (`denyOnError`, `limit` 0, response headers, `sourceCriterion`).

### Benchmarks

Measured at commit `a51d20a3` on 2026-11-21 on an Intel Core Ultra 7 265K (20 threads), Windows 11, `go1.25.6 windows/amd64`, with Yaegi v0.16.1 (the version Traefik v3.7.11 pins). Each value is the median of five runs:

```text
go test -run '^$' -bench . -benchtime 1s -count 5 ./tokenbucket/ ./windowcounter/
```

`go.mod` declares `go 1.21`; these numbers are from go1.25.6 only. CI compiles the benchmarks but does not run them.

How to read the numbers:

- The Redis rows talk over TCP loopback to an in-process fake Redis (`fake_redis_test.go` in each package) with a warm script cache, so each `Eval` is one `EVALSHA`. Against an engine that does not have the script yet, the first call per script costs one extra `EVAL`. Loopback on Windows dominates these rows: compare them with each other, not with the latency of a real Redis.
- `redis-cmds/op` is the commands the fake served per operation, including the background flush.
- The fake serves one command at a time, so the `-parallel` rows against it measure the client plus a serialized server, not how Redis scales. `BenchmarkLiveRedisAllow` and `BenchmarkLiveTake` (`-bench Live` with `TOKENBUCKET_LIVE_*` / `WINDOWCOUNTER_LIVE_*` set) run against a real engine; they are not in this table.
- Yaegi rows run the same code interpreted, one hot key, through a probe loop inside one `Eval`. They have no allocation column.
- `sync_rate` 20 ms is the floor, the buffered setting with the most Redis traffic. `BenchmarkFlush` times one flush of 1000 dirty keys.
- The "before" column is the same benchmark at commit `d940bf43`, before the two optimizations in this repo: a new source in `tokenbucket.NewMemory` used to walk every stored source under the mutex, and the buffered flush sent one `EVAL` per key. Check out that commit and run the same command to reproduce it.
- One row without a "before" value also moved: `windowcounter` at `sync_rate` 20 ms with 10k rotating keys went from 76,738 to 62,485 ns/op and from 3.01 to 2.0 `redis-cmds/op`. The background flush used to send one `EVAL` per dirty key; it now sends one per batch of up to 256. The two commands left per Take are `GET`s of the current and previous window, because each key comes back more than one `sync_rate` after its last flush.

| Benchmark (sub-benchmark) | Compiled ns/op | Yaegi ns/op | redis-cmds/op | Compiled allocs/op | Before (commit `d940bf43`) ns/op |
|---|---:|---:|---:|---:|---:|
| `tokenbucket.NewMemory`, hot key (`BenchmarkMemoryAllow/hot-key`) | 21.9 | 8,567 | — | 0 |  |
| `tokenbucket.NewMemory`, 10k rotating keys (`BenchmarkMemoryAllow/rotating-10k`) | 38.0 | — | — | 0 |  |
| `tokenbucket.NewMemory`, new key, 65536 live sources (`BenchmarkMemoryAllow/new-key-full-table`) | 189 | — | — | 2 | 1,516,150 |
| `tokenbucket.NewMemory`, hot key, parallel (`BenchmarkMemoryAllow/hot-key-parallel`) | 59.2 | — | — | 0 |  |
| `tokenbucket.NewRedis`, hot key (`BenchmarkRedisAllow/hot-key`) | 34,353 | 182,373 | 1 | 74 |  |
| `tokenbucket.NewRedis`, 10k rotating keys (`BenchmarkRedisAllow/rotating-10k`) | 34,136 | — | 1 | 72 |  |
| `tokenbucket.NewRedis`, hot key, parallel (`BenchmarkRedisAllow/hot-key-parallel`) | 22,783 | — | 1 | 77 |  |
| `windowcounter`, `sync_rate` 0, hot key (`BenchmarkTake/sync-rate-0/hot-key`) | 57,683 | 284,752 | 2 | 68 |  |
| `windowcounter`, `sync_rate` 0, 10k rotating keys (`BenchmarkTake/sync-rate-0/rotating-10k`) | 58,085 | — | 2 | 67 |  |
| `windowcounter`, `sync_rate` 0, hot key, parallel (`BenchmarkTake/sync-rate-0/hot-key-parallel`) | 26,238 | — | 2 | 75 |  |
| `windowcounter`, `sync_rate` 20 ms, hot key (`BenchmarkTake/sync-rate-20ms/hot-key`) | 240 | 18,108 | 3.1e-05 | 4 |  |
| `windowcounter`, `sync_rate` 20 ms, 10k rotating keys (`BenchmarkTake/sync-rate-20ms/rotating-10k`) | 62,485 | — | 2 | 67 |  |
| `windowcounter`, `sync_rate` 20 ms, hot key, parallel (`BenchmarkTake/sync-rate-20ms/hot-key-parallel`) | 204 | — | 0.00025 | 4 |  |
| `windowcounter` flush, 1000 dirty keys (`BenchmarkFlush/dirty-keys-1000`) | 907,582 | — | 4 | 15941 | 31,947,335 |

## Backend backoff

Package `backendbackoff/`. An in-memory per-key admission gate: stop forwarding into an unhealthy backend, then back off exponentially while it recovers. It is not Traefik's CircuitBreaker middleware and not a token bucket.

`Allow` says whether a real backend attempt may proceed and how long to wait if not. After an admitted attempt, `Report` the boolean outcome. The library does not sleep, does not write HTTP, and does not classify status codes. Denied requests must not be Reported.

```go
gate, err := backendbackoff.New(backendbackoff.Config{})
if err != nil {
	return err
}
allowed, retryAfter, err := gate.Allow(req.Context(), "backend:"+host)
if err != nil {
	return err
}
if !allowed {
	return errLimited
}
_ = retryAfter
ok := callBackend()
gate.Report("backend:"+host, ok)
```

Prefix keys in the caller. Store the Gate in a reclaim table the caller owns so a Traefik reload keeps the map (`Close` as the reclaim hook; no Sleep/Wake). Two Gate instances do not share state. Do not import `tokenbucket`.

## CIDR lookup

Package `iplookup/`. An in-process CIDR store: IPv4 and IPv6 prefixes live on separate trees so a prefix cannot match the other family. Callers pass `net.IP` they already selected. The library does not read HTTP.

```go
h := iplookup.New()
if err := h.AddCIDR("10.0.0.0/8", "office"); err != nil {
	return err
}
found, prefixLen, label, err := h.Contains(net.ParseIP(ip))
if err != nil {
	return err
}
_ = prefixLen
if found && label == "office" {
	return nil
}
```

The same canonical prefix stored again replaces the label. `RemoveCIDR` drops one prefix. `Reset` clears both families.

For config lists that mix CIDRs and single addresses, use `AddEntry`. A bare address is stored as its host prefix (`HostCIDR`): `10.0.0.1/32`, `2001:db8::1/128`, and an IPv4-mapped `::ffff:10.0.0.1` as the IPv4 host `10.0.0.1/32`.

```go
for _, entry := range cfg.TrustedProxies { // "10.0.0.0/8", "192.0.2.10", "2001:db8::1"
	if err := h.AddEntry(entry, "proxy"); err != nil {
		return err
	}
}
```

## Traefik emulator

Package `traefikemulator/`. A stdlib test helper that stands in for Traefik `RouterFactory.CreateRouters`. One configuration generation at a time: `Apply` cancels the previous context, constructs each route with your plugin `New`, and `Serve` / `Handler` hit the current generation by route name. No live Traefik, Redis, or Yaegi — compiled `go test` only.

```go
emu := traefikemulator.New(myPluginNew)
t.Cleanup(emu.Stop)
if failed := emu.Apply([]traefikemulator.Route{{Name: "mw", Next: next, Config: cfg}}); failed != nil {
	t.Fatal(failed)
}
rec := httptest.NewRecorder()
if !emu.Serve("mw", rec, httptest.NewRequest(http.MethodGet, "/", nil)) {
	t.Fatal("route missing")
}
```

Constructor errors omit that route and leave siblings on the same generation context. Duplicate route names in one `Apply` fail without replacing an earlier success.

## Yaegi

This code runs interpreted inside Traefik when a middleware is loaded as a plugin, and compiled when the middleware is built into a custom Traefik binary. The interpreted path is the constraint: treat it as a hard rule, not a later port.

- Prefer the Go standard library. Every import is also interpreted.
- No cgo, no `unsafe`, no assembly, no `//go:embed` unless Yaegi in the target Traefik version is known to support it.
- Avoid features the Traefik Yaegi build does not implement (some generics, some `reflect`, runtime-heavy packages).
- Keep APIs simple: concrete types, explicit methods, no clever init-time magic.
- Prove each package with tests that a normal `go test` run covers, then sanity-check the same sources under Yaegi before calling a library done.

If a change would be fine in compiled Go but fails under Yaegi, the Yaegi failure wins.

## Layout

```text
reclaim/         reclaim table
simpleredis/     stdlib Redis client (Apache-2.0)
windowcounter/   sliding-window hit counter
tokenbucket/     Traefik token bucket (in-process and Redis)
backendbackoff/  in-memory backend backoff gate
iplookup/         CIDR lookup helper (family-isolated)
traefikemulator/  Traefik router factory test helper (stdlib)
e2e/             fake Traefik plugins + Pester harness (Yaegi)
```

Module path: `github.com/david-garcia-garcia/traefik-middleware-utilities`.

## Tests

Four suites (see `knowledge/devdocs/std_go_test-suites.md`):

```text
go test -short ./...          # unit Go — fake TCP, no Redis (CI job Unit; TestAlloc* run)
go test -race -short ./...   # same suite under the detector (CI job race; needs gcc)
go test ./...                # also Go E2E for each *_LIVE_REDIS / *_LIVE_DRAGONFLY addr that is set
./Test-Integration.ps1                         # Pester reclaim then SimpleRedis redis then dragonfly
./Test-Integration.ps1 -Suite reclaim             # CI Integration Tests
./Test-Integration.ps1 -Suite simpleredis -Engine redis
./Test-Integration.ps1 -Suite simpleredis -Engine dragonfly
```

`Test-Integration.ps1` starts Traefik v3.7.11 with fake local plugins (`e2e/reclaimprobe`, `e2e/simpleredisprobe`) so reclaim and SimpleRedis run under Yaegi. Docker is required. `-Suite reclaim` is reclaim only; `-Suite simpleredis -Engine redis|dragonfly` runs the SimpleRedis file once against that backend.

Go E2E files are `{domain}_e2e_test.go` next to that domain (`commands_e2e_test.go`, `limiter_e2e_test.go`). They skip under `-short` or when both live addrs are unset. One addr set runs that engine only. Set `SIMPLEREDIS_LIVE_*`, `WINDOWCOUNTER_LIVE_*`, and `TOKENBUCKET_LIVE_*` for the engines to hit (Redis `:6379`, Dragonfly `:6380`). Passworded AUTH proof uses `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` (`:6381` / `:6382`). `backendbackoff/` and `iplookup/` have no store and no `*_LIVE_*` var.

The rate limiter benchmarks (`bench_test.go`, `bench_yaegi_test.go`, `bench_e2e_test.go` in `tokenbucket/` and `windowcounter/`) compile in CI but do not run there; the command and the results are in `## Rate limiters compared`.

CI (`.github/workflows/ci.yml`) runs golangci-lint, unit `go test -short` (no engines, 2m timeout, `TestAlloc*` run), unit `go test -race -short` (no engines, 10m timeout), Go E2E Redis (`go test` with Redis 7, no `-race`), Go E2E Dragonfly (`go test` with Dragonfly, no `-race`), Pester reclaim (`Integration Tests`), Pester SimpleRedis Redis, and Pester SimpleRedis Dragonfly on every pull request and on pushes to `master`.

Tag a version (`v1.0.0`) to cut a GitHub release via GoReleaser (source archive + SBOM; no plugin binary).
