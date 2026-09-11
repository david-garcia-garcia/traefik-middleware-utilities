# Requirement
IssueKey: 2026-09-11-simpleredis-perf-02-io-timeout

## Problem
SimpleRedis timing knobs are compile-time constants (`dialTimeout` 2s, `ioTimeout` 1s, `idleTimeout` 30s, `maxIdleConns` 8). Callers cannot set shorter deadlines. A slow Redis holds each in-flight command for up to 1s (I/O) or 2s (dial), then every new request dials another socket. Planned consumers (rate limiters, cache) need tens of milliseconds then fallback.

## Current (code)
- `simpleredis/simpleredis.go:25-30` — package constants `maxIdleConns=8`, `idleTimeout=30s`, `dialTimeout=2s`, `ioTimeout=1s`. No exported options type.
- `simpleredis/simpleredis.go:53-62` — `SimpleRedis` stores `host`, `pass`, `database` only; no timeout/pool fields.
- `simpleredis/simpleredis.go:80-85` — `Init(host, pass, database)` stores those three strings; not mutex-protected; does not dial.
- `simpleredis/simpleredis.go:263-265` — `dial` uses `net.Dialer{Timeout: dialTimeout}` (the constant).
- `simpleredis/simpleredis.go:291-293` — `do` applies one `SetDeadline(now+ioTimeout)` covering write and read.
- `simpleredis/simpleredis.go:181-191` — `exec` does not retry `errTimeout`.
- `simpleredis/simpleredis.go:203-241` — `borrow` reuses idle younger than `idleTimeout`, else dials with no live-socket cap or wait queue.
- `simpleredis/simpleredis.go:244-260` — `release` closes when idle list length ≥ `maxIdleConns`.
- `simpleredis/simpleredis.go:431-436` — `ioError` maps `os.ErrDeadlineExceeded` to `redis:timeout` via `errors.Is`.
- `simpleredis/simpleredis_test.go:588-608` — `TestIoTimeout` fake that never replies; asserts `redis:timeout` on the constant 1s deadline.
- `e2e/simpleredisprobe/plugin.go:25-58` — probe `Config` has `Host` only; `New` calls `Init(host, "", "")`.
- `docker-compose.yml:47-75` — `redis:7-alpine` at `redis:6379`, Dragonfly `v1.40.2` at `dragonfly:6379`; `/redis` and `/dragonfly` routes.
- `Test-Integration.ps1` + `scripts/integration-tests.Tests.ps1:84-114` — Pester happy-path verbs on `/redis` and `/dragonfly`; no timeout/pool-knob proof.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — Init is three strings; dial timeout SHALL be two seconds; idle pool at most eight; idle older than thirty seconds not reused; I/O deadline → `redis:timeout`.
- `InitWithOptions` / exported timeout fields — not found.

## Desired
- Make dial timeout, I/O timeout, idle timeout, and idle cap configurable; keep today's numbers as zero-value defaults so `Init(host, pass, database)` is unchanged.
- Surface is Yaegi-safe: plain struct/primitives (extend `Init`, `InitWithOptions`, or exported fields set before first use). No functional-option closures.
- If `poolSize` / `poolTimeout` appear as config fields (finding names them), add the knobs and document that the wait-queue semaphore belongs to perf-01; do not implement that rewrite here.
- Do not add retries around timeout.
- Separate read vs write deadlines are optional; one combined `SetDeadline` is enough.
- `TestIoTimeout` keeps passing on defaults. Add a fake-server test that a *configured* I/O timeout fires instead of the constant.
- **Live proof (caller):** fake-server tests are not a substitute. Prove the new knobs on **both** Redis and Dragonfly via dest compose + Pester (`Test-Integration.ps1`, `/redis` and `/dragonfly`, `e2e/simpleredisprobe`) and/or live Go tests. Lua 5.1-safe. Dragonfly KEYS required. CI must exercise both backends.

## Affected
- `simpleredis/simpleredis.go` (Init/options, `dial`, `do`, `borrow`, `release`)
- `simpleredis/simpleredis_test.go`, `simpleredis/yaegi_test.go`
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`
- `e2e/simpleredisprobe/plugin.go` (probe Config if e2e must set knobs)
- `docker-compose.yml`, `Test-Integration.ps1`, `scripts/integration-tests.Tests.ps1`, `.github/workflows/ci.yml` (CI already runs both backends; must keep doing so for the new proof)
- `knowledge/devdocs/std_go_simpleredis.md`

## Out of scope
- perf-01 wait-queue pool rewrite (buffered semaphore, live-socket cap behavior).
- perf-03 idle-reaper, pipelining, EVALSHA, encoding/decoding, MSETEX, other index findings.
- Retries on `errTimeout`.
- go-redis, miniredis, TLS, Unix sockets, functional options.

## Unknowns
- Options surface: `InitWithOptions`, extra `Init` args, or exported fields on `SimpleRedis`.
- How to stall a live Redis **and** Dragonfly command long enough to prove a configured I/O timeout without `DEBUG SLEEP` (Dragonfly may lack it) while staying Lua 5.1-safe and KEYS-declared.
- Whether unused `poolSize`/`poolTimeout` fields are stored as no-ops or omitted from the struct and only documented.
- Whether the Traefik probe Config must expose the new timeouts for Pester, or live Go tests against compose ports are enough.

## Tensions
- Finding How to fix lists `poolSize`/`poolTimeout` next to the timeout knobs and asks a `poolSize` live-socket bound test; caller forbids taking perf-01's semaphore — add knobs (or document them) and leave the wait queue to perf-01.
- Index suggested order says do perf-01 and perf-02 together; this ticket is bound to perf-02 only.
- Finding How to prove is a never-reply fake server; caller requires live Redis and Dragonfly proof. Do both: keep/extend fake tests and add live proof.
- Spec `std_go_simpleredis_tcp-session` hardcodes two-second dial, eight idle, thirty-second idle age; configurable knobs will need a spec delta so defaults stay those numbers when unset.
