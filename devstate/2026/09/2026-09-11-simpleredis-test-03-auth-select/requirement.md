# Requirement
IssueKey: 2026-09-11-simpleredis-test-03-auth-select

## Problem
`dial`'s AUTH and SELECT failure branches (`conn.close()` then return the error) have zero test coverage. `TestRejectedAuthIsReturned` Inits with an empty password, so the AUTH block never runs and `-NOAUTH` answers GET. `TestAuthAndSelectOncePerDial` covers only a successful handshake. A wrong password or bad DB index is unproven: close, do not pool, surface `redis:noauth` on AUTH-class prefixes, one error (no `exec` retry storm).

## Current (code)
- `simpleredis/simpleredis.go` `dial` (`:262-289`) — if `pass != ""` sends AUTH then on error `conn.close()` and returns; if `database != ""` sends SELECT then on error `conn.close()` and returns. AUTH before SELECT (`:275`).
- `simpleredis/simpleredis.go` `replyError` (`:419-428`) — prefixes `NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH` → `redis:noauth`; other `-` text is `errors.New`.
- `simpleredis/simpleredis.go` `exec` (`:181-201`) — dial errors from `borrow` return immediately (`:184-186`); retry only when a reused idle socket is dead.
- `simpleredis/simpleredis_test.go` `startFakeRedis` `serve` (`:63-68`) — AUTH and SELECT always `+OK`; handshake replies are not configurable.
- `simpleredis/simpleredis_test.go` `startStaticRedis` (`:229-256`) — same canned reply for every command, including GET.
- `simpleredis/simpleredis_test.go` `TestRejectedAuthIsReturned` (`:406-421`) — `Init(addr, "", "")` then Get; four AUTH-class prefixes mapped on the command reply, not handshake.
- `simpleredis/simpleredis_test.go` `TestAuthAndSelectOncePerDial` (`:524-541`) — `Init(addr, "secret", "2")`; success only (1 AUTH, 1 SELECT, 3 GET).
- `e2e/simpleredisprobe/plugin.go` — `Config` has `Host` only; `Init(host, "", "")`.
- `docker-compose.yml` — `redis:7-alpine` and `dragonfly:v1.40.2` with no password and no extra databases; probe routes `/redis` and `/dragonfly`.
- `scripts/integration-tests.Tests.ps1` — Pester success-path verb headers on those two routes; no wrong-password or bad-SELECT case.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — AUTH/SELECT once per dial on success; no failure-branch scenarios.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — compose SHALL be no-password empty database on both engines; AUTH-class prefix mapping is specified for command replies.
- `knowledge/research/index_ext_redis.md` / `index_ext_dragonfly.md` — INCR/EXPIRE/EVAL and Dragonfly image; no AUTH/SELECT live-error finding.

## Desired
- In-process fake whose handshake replies are configurable (finding): AUTH rejected with `Init(addr, "wrong-password", "")` for each `replyError` prefix (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`); assert `redis:noauth`, empty `sr.idle`, peer saw close.
- SELECT rejected: fake answers AUTH `+OK` (when AUTH is sent) and SELECT `-ERR DB index is out of range`; error reaches the caller; nothing pooled; AUTH ran first when a password is set.
- No retry storm: one handshake failure surfaces one error; `exec` must not loop redials.
- Coverage of `dial` blocks `277.85,280.4` and `283.91,286.4` becomes non-zero; tests fail if `conn.close()` is removed or a usable conn is returned.
- Tests MUST run against both Redis and Dragonfly. Handshake AUTH/SELECT failure branches need that fake AND live proof on both engines where the engine supports AUTH/SELECT (wrong password / bad DB index). Redis and Dragonfly may differ; still run the live cases each engine supports.
- Extend compose + Pester if dest's harness can take a passworded service.
- Any Eval the harness still sends stays Lua 5.1-safe with Dragonfly KEYS required.

## Affected
- `simpleredis/simpleredis_test.go` (fake variant + handshake-failure tests)
- `e2e/simpleredisprobe/` and `docker-compose.yml` and `scripts/integration-tests.Tests.ps1` if a passworded/bad-SELECT live path is added
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (failure scenarios; propose)
- possibly `openspec/specs/std_go_simpleredis_resp-commands/spec.md` if compose no-password SHALL is extended

## Out of scope
- Changing `dial` / `replyError` / pool / timeout production logic unless a test proves it is wrong.
- Other simpleredisfixes (test-01/02/04–07, perf-*, feat-01).
- EVALSHA, SCRIPT LOAD, pipelining, go-redis, miniredis, TLS, Unix sockets.
- Reclaim e2e routes `/a` `/b`.
- New Lua scripts beyond the existing KEYS-declared probe snippet.

## Unknowns
- Exact AUTH/SELECT error strings on live `redis:7-alpine` vs `dragonfly:v1.40.2` (research indexes do not cover AUTH/SELECT).
- Whether Dragonfly supports `requirepass` / wrong-password and `SELECT` out-of-range; which live cases each engine can run.
- Whether dest's compose/Pester can take a passworded service without breaking the existing no-password `/redis` and `/dragonfly` success routes (separate services vs extra probe Config fields).
- How the fake observes socket close (finding asks for it; current fake has no close counter).

## Tensions
- Finding How-to-fix is fake-only; conductor requires live Redis and Dragonfly as well — take both.
- Finding SELECT case says `Init(addr, "", "99")` plus “AUTH did succeed first”; with empty pass AUTH is skipped. Passworded SELECT-fail is what pins AUTH-before-SELECT.
- `TestRejectedAuthIsReturned` name vs handshake coverage; keep command-reply mapping tests, add real handshake tests.
- `std_go_simpleredis_resp-commands` compose SHALL no-password empty database vs conductor “extend compose if the harness can take a passworded service”.
