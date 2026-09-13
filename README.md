# traefik-middleware-utilities

[![Build Status](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/workflows/ci.yml/badge.svg)](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/david-garcia-garcia/traefik-middleware-utilities)](https://goreportcard.com/report/github.com/david-garcia-garcia/traefik-middleware-utilities)
[![Latest GitHub release](https://img.shields.io/github/v/release/david-garcia-garcia/traefik-middleware-utilities?sort=semver)](https://github.com/david-garcia-garcia/traefik-middleware-utilities/releases/latest)
[![License](https://img.shields.io/badge/license-Apache%202.0-brightgreen.svg)](LICENSE)

Shared Go libraries for Traefik middlewares.

## Why this exists

Traefik loads local and catalog plugins through [Yaegi](https://github.com/traefik/yaegi), an interpreter. It does not compile your plugin into Traefik.

That is why this repo exists. The official Go Redis client (and other compiled-only libraries) will not run under Yaegi. A middleware that imports them cannot be loaded the way Traefik actually loads plugins, and cannot be tested that way either.

These packages are the pieces those middlewares otherwise rewrite: a Redis client, a reclaim table, two rate-limit clocks, and an in-memory backend backoff gate. They stay in the subset of Go that Yaegi can interpret, and they are tested under Yaegi, not only as compiled Go.

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

`sync_rate` `0` talks to Redis on every Take or Peek and returns that call's Redis error. A positive duration batches locally and flushes on that interval; a failed flush is retained, and after one missed `sync_rate` Take/Peek return that error instead of a silent nil. Pass `Sleep` / `Wake` / `Close` into reclaim when the counter is the stored value. Do not close the Redis client from the counter.

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

## Yaegi

This code is interpreted inside Traefik, not compiled into it. Treat that as a hard constraint, not a later port.

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

Go E2E files are `{domain}_e2e_test.go` next to that domain (`commands_e2e_test.go`, `limiter_e2e_test.go`). They skip under `-short` or when both live addrs are unset. One addr set runs that engine only. Set `SIMPLEREDIS_LIVE_*`, `WINDOWCOUNTER_LIVE_*`, and `TOKENBUCKET_LIVE_*` for the engines to hit (Redis `:6379`, Dragonfly `:6380`). Passworded AUTH proof uses `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` (`:6381` / `:6382`). `backendbackoff/` has no store and no `*_LIVE_*` var.

CI (`.github/workflows/ci.yml`) runs golangci-lint, unit `go test -short` (no engines, 2m timeout, `TestAlloc*` run), unit `go test -race -short` (no engines, 10m timeout), Go E2E Redis (`go test` with Redis 7, no `-race`), Go E2E Dragonfly (`go test` with Dragonfly, no `-race`), Pester reclaim (`Integration Tests`), Pester SimpleRedis Redis, and Pester SimpleRedis Dragonfly on every pull request and on pushes to `master`.

Tag a version (`v1.0.0`) to cut a GitHub release via GoReleaser (source archive + SBOM; no plugin binary).
