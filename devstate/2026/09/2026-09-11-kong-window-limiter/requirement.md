# Requirement
IssueKey: 2026-09-11-kong-window-limiter

## Problem
`master` has SimpleRedis with INCR/EXPIRE/EVAL and reclaim lifecycle hooks, but no distributed window rate limiter. README still plans a leaky `bucket/` primitive. Middlewares need a Kong-style sliding-window counter (Redis/Dragonfly-backed, optional buffered `sync_rate`) that is Yaegi-safe and testable without Traefik HTTP probes.

## Current (code)
- `ratelimit/` — not found (handoff name is `windowcounter/`).
- `bucket/` — not found; README lists leaky bucket as planned (`README.md`).
- `simpleredis/simpleredis.go` — `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `Close`; errors as `redis:unreachable` / `redis:timeout` text.
- `simpleredis/simpleredis_test.go` `startFakeRedis` — in-process RESP fake for unit tests; includes `kongIncrbyExpireatScript` (INCRBY + conditional EXPIREAT flush pattern).
- `simpleredis/yaegi_test.go` — Yaegi probe for Init/Get/Set/Del/Incr/Eval against compiled fake only.
- `reclaim/table.go` — `Hooks{Sleep, Wake, Close}` lifecycle; `Reset` runs Sleep then Close on stored values.
- `docker-compose.yml` — `redis:7-alpine` and Dragonfly `v1.40.2` services; used by Traefik Pester harness, not by a direct `go test` live suite.
- `.github/workflows/ci.yml` — `go test -v ./...` plus Pester `./Test-Integration.ps1`; no job that starts Redis/Dragonfly for package-level live limiter tests.
- `e2e/simpleredisprobe/plugin.go` — Traefik probe exercises SET/GET/Incr/Eval headers; not a rate limiter.
- `knowledge/devdocs/std_go_simpleredis.md` — documents SimpleRedis commands; no rate-limit usage packet.
- `knowledge/research/ext_redis_*`, `ext_dragonfly_eval/` — Redis/Dragonfly command facts; no Kong sliding-window research before this prepare.

## Desired
- New `windowcounter/` package: opaque key + limit + window; `Take`/`Allow` returns allowed + usage; sliding window only (`estimated = current + previous × (1 − elapsed/window)`).
- `sync_rate=0`: synchronous `Incr` every `Take`; expire on first hit in window.
- `sync_rate>0`: admit from `redis_known + local_delta`; periodic timer flushes via `Eval` INCRBY + EXPIREAT-if-new (Kong OSS flush shape); min interval ~20ms floor.
- Redis/Dragonfly via `simpleredis` only; propagate `redis:unreachable` / timeout; no fail-open/fail-close or health gate inside library.
- Flush goroutine tied to reclaim-style `Sleep`/`Wake`/`Close` so Traefik reload stops tickers cleanly.
- README library row becomes this primitive (not leaky bucket).
- **Unit tests:** fake TCP (SimpleRedis fake style) for window math and encoder paths without Docker.
- **Live Go e2e:** table-driven suite against real Redis and Dragonfly (`Init` + `Take` on ports); prove exact mode, buffered multi-client sharing, sliding boundary (no fixed-window double), both backends.
- **Yaegi live:** same scenarios interpreted (stdlib only, `useunsafe` false, GOPATH copy); compiled test owns engine start/skip; CI must start both engines and not skip.
- Pester/Traefik plugin optional; not a substitute for the Go live suite.

## Affected
- `windowcounter/` (new)
- `README.md` (library table and layout)
- `.github/workflows/ci.yml` (live Redis/Dragonfly for limiter tests)
- `docker-compose.yml` or test harness wiring for live addrs (explore)
- `knowledge/devdocs/` (new usage packet during later phases)
- `openspec/specs/` (new spec leaf during propose)

## Out of scope
- Token bucket, leaky bucket, Traefik token-bucket Lua, in-memory-only limiter product.
- Fixed window v1.
- HTTP, 429 responses, sleep/throttle in library.
- `go-redis`, GET/SET counter races, EVALSHA.
- Fail-open/fail-close on Redis errors; health gate.
- Prefixing/namespacing rate-limit keys (caller's job).

## Unknowns
- Exact Redis key scheme for current vs previous window counters (ticket gives formula only).
- Whether flush timer registers through `reclaim.Table` hooks or a standalone `Sleep`/`Wake`/`Close` interface on the limiter type.
- How live tests discover Redis/Dragonfly addrs in CI (compose hostnames vs env vs testcontainers) while keeping `-short` skip locally.
- Whether one limiter instance owns one `SimpleRedis` client or callers inject shared clients for multi-instance `sync_rate>0` tests.

## Tensions
- README and layout still name `bucket/` leaky bucket as planned; ticket locks `ratelimit/` and forbids leaky/token buckets — README row must change to this primitive.
- Prior tickets extended Pester for SimpleRedis; this ticket makes Pester optional and requires direct `go test` live suite as the behaviour proof — CI shape differs from existing integration job.
