# Requirement
IssueKey: 2026-09-11-simpleredis-test-01-eof-redial

## Problem
The production stale-connection path — Redis or Dragonfly closes the TCP socket while SimpleRedis still holds it idle — is untested. `TestStaleConnectionIsRetried` closes the pooled socket from the client, so `SetDeadline` fails with `os.ErrClosed` and never reaches `ioError`'s mapping of `io.EOF` to `redis:unreachable`. Compose/Pester only prove happy-path verbs on `/redis` and `/dragonfly`, not idle/server-close recovery.

## Current (code)
- `simpleredis/simpleredis.go` `ioError` (431-437) — `os.ErrDeadlineExceeded` → `errTimeout`; every other IO error (including `io.EOF`) → `errUnreachable`.
- `simpleredis/simpleredis.go` `do` (292-307) — `SetDeadline` failure → `errUnreachable` without `ioError`; `writeCommand` failure → `ioError`; unclean `readReply` IO → `ioError`.
- `simpleredis/simpleredis.go` `exec` (182-201) — one retry when a reused idle conn is dead and the error is not timeout; second `borrow` failure returns that error (`:195-197`).
- `simpleredis/simpleredis.go` `release` (244-260) — `reusable == false` closes the socket and does not append to `idle`.
- `simpleredis/simpleredis.go` (27) — client `idleTimeout` is 30s, so a server-closed socket younger than 30s is still borrowed.
- `simpleredis/simpleredis_test.go` `TestStaleConnectionIsRetried` (468-493) — client-side `conn.close()` on idle sockets; second Get and 2 accepts. Does not close from the peer.
- `simpleredis/simpleredis_test.go` `startFakeRedis`/`serve` (28-54) — serves until read error; does not close the accepted socket after the first reply.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — "A dead pooled connection SHALL be retried once unless the error is a timeout." No peer-closed / `io.EOF` scenario.
- `e2e/simpleredisprobe/plugin.go` — `Init` in `New`; each request runs verbs against `Config.Host`. No idle or server-close step.
- `docker-compose.yml` — `redis:7-alpine` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`; no idle-client timeout. Routes `/redis` and `/dragonfly`.
- `scripts/integration-tests.Tests.ps1` — Pester `/redis` and `/dragonfly` assert verb headers only; no request after a server-side close.
- `.github/workflows/ci.yml` — `test` job already runs Redis `:6379` and Dragonfly `:6380` for windowcounter/tokenbucket live env; `simpleredis/` has no live tests on those addrs.
- `windowcounter/live_test.go` — dest Go live-engine pattern (env addrs, skip if unset / `-short`). Not used by `simpleredis/`.

## Desired
- Fake TCP server that accepts, answers the first command, then **closes the socket from the server** without reading further (do not close the client). Optionally accept a second connection and serve it. First Get succeeds; second Get succeeds on a new dial; fake saw 2 accepts; dead conn is not in `sr.idle`.
- Variant with no second accept (listener closed) so retry `borrow` fails; error is `redis:unreachable` (covers `:195-197`).
- Rename `TestStaleConnectionIsRetried` to name the `SetDeadline` / `os.ErrClosed` arm it actually covers.
- Coverage of `simpleredis.go:436` becomes non-zero; inverted retry at `:190` or pooling a non-reusable conn fails the new test.
- Test coverage MUST run against both Redis and Dragonfly.
- Also prove a live idle/server-close recovery path against both Redis and Dragonfly using compose services; do not skip Dragonfly.
- Extend Pester `/redis` `/dragonfly` and `e2e/simpleredisprobe` (dest's live Traefik harness) so that recovery is visible there.
- Any Lua/Eval in this work: Lua 5.1-safe; keys listed in `KEYS` (Dragonfly).
- Add a tcp-session spec scenario for peer-closed idle socket (`io.EOF` → one retry) distinct from client-side close. Do not change the retry policy unless a new test proves dest is wrong.

## Affected
- `simpleredis/simpleredis_test.go` (`simpleredis.go` only if tests expose a retry/`clean` bug)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`
- `e2e/simpleredisprobe/`
- `scripts/integration-tests.Tests.ps1`
- `docker-compose.yml` if timeout/kill wiring is required
- `.github/workflows/ci.yml` and/or a `simpleredis` live test using dest Redis/Dragonfly services

## Out of scope
- perf-01 / perf-02 / perf-03 (pool cap, I/O timeout fan-out, idle reaper).
- Standalone coverage of `writeCommand` internal write-error branches (`:311-323`); the asked path is peer close, where the write often succeeds and the read returns `io.EOF`.
- Changing timeout non-retry (`TestTimeoutOnReusedConnIsNotRetried` already covers it).
- EVALSHA, pipelining, MSETEX, rate limiters, miniredis, go-redis.
- Using client-side `conn.close()` as the new proof (existing test already does that).

## Unknowns
- Exact live close that both engines support in compose/CI: Redis `timeout`, Dragonfly equivalent, `CLIENT KILL`, or container restart. Research indexes cover INCR/EVAL/image, not idle-client timeout.
- Whether CI `test` services (host 6379/6380) plus a Go live test, compose+Pester, or both are required to "not skip Dragonfly"; dest already has both harnesses.
- Wait vs client `idleTimeout` (30s) so the pooled conn is still borrowed after the server closed it.
- Whether the probe must expose reuse/redial in headers, or Pester can infer recovery from a second 200 after a kill.

## Tensions
- Finding How to fix is compiled fake TCP only; conductor HARD REQUIREMENT also requires live Redis and Dragonfly recovery and extending Pester/probe — take both.
- Finding "measured coverage" also names `writeCommand` failure branches; How to fix does not ask to unit-test those in isolation — leave them out of scope.
- Spec already requires one retry of a dead pooled conn; dest tests only the client-close arm — this ticket adds the peer-close arm.
- Index groups test-01 with test-02..05; this ticket is test-01 only.
