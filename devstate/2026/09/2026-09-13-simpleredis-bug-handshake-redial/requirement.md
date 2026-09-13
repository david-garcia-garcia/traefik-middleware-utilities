# Requirement
IssueKey: 2026-09-13-simpleredis-bug-handshake-redial

## Problem
Handshake AUTH or SELECT failure returns from `dial` unchanged. `exec` retries when `shouldRetry` is true (`errUnreachable` from EOF, `LOADING ` prefix, exact `ERR max number of clients reached`), so `Get` with `MaxRetries: 1` opens a second TCP connection. Spec `std_go_simpleredis_tcp-session` “Handshake AUTH or SELECT failure” requires one error and MUST NOT open a second TCP connection.

## Current (code)
- `simpleredis/pool.go` `dial` — after TCP `DialContext`, AUTH then SELECT via `do`. On AUTH/SELECT error: `conn.close()` and `return nil, err` with that `do` error unchanged. TCP `DialContext` failure returns unmarked `errUnreachable` (or `contextStop`).
- `simpleredis/pool.go` `borrow` — idle miss calls `dial`; that error is returned to `exec` after `freeInUseTurn`.
- `simpleredis/commands_exec.go` `exec` — `borrow` error: if `shouldRetry` and not closed, continue (another dial). Same after `do` on the command path.
- `simpleredis/commands_exec.go` `shouldRetry` — false for cancel, deadline, `errTimeout`. True for `err == errUnreachable`, then `isRetryableRedisReply`.
- `simpleredis/commands_exec.go` `isRetryableRedisReply` — true for exact `ERR max number of clients reached` and prefixes `LOADING `, `READONLY `, `MASTERDOWN `, `CLUSTERDOWN `, `TRYAGAIN `.
- `simpleredis/resp.go` `ioError` — deadline → `errTimeout`; other IO including EOF → `errUnreachable`. Shared by AUTH/SELECT and GET.
- `simpleredis/resp.go` `replyError` — `NOAUTH`/`WRONGPASS`/`NOPERM`/`ERR Client sent AUTH` → `errNoAuth`. Other Redis errors are `errors.New(text)` (LOADING, max-clients, `ERR DB index is out of range`).
- `simpleredis/pool_test.go` `TestHandshakeAuthRejectedMapsToNoAuthAndIsNotPooled` — WRONGPASS/NOAUTH/NOPERM/`ERR Client sent AUTH` already 1 accept (`setHandshakeReplies`).
- `simpleredis/pool_test.go` `TestHandshakeSelectRejectedAfterAuthIsNotPooled` — AUTH OK then SELECT `ERR DB index is out of range`, 1 accept.
- `simpleredis/commands_exec_test.go` `TestLoadingReplyIsRetried` — GET `-LOADING …` then success (command path, not handshake). No accept-count assert.
- `simpleredis/fake_redis_test.go` `startFakeRedis`, `setHandshakeReplies`, `connections()` — AUTH close-without-reply is not a handshake mode; `armCloseBeforeReplyOnceForTest` is the next command after handshake.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — handshake failure SHALL surface one error and MUST NOT open a second TCP connection. Scenarios cover AUTH-class prefixes and SELECT 99, not AUTH EOF / SELECT EOF / AUTH LOADING / AUTH max-clients.
- Dest has no `//go:build bugrepro` handshake redial tests under `simpleredis/`.

## Desired
1. FIRST land compiled tests that reproduce on dest and MUST fail there. No `//go:build bugrepro`. They run under default `go test -short ./simpleredis/`. Prefer `simpleredis/pool_test.go`. Reuse `startFakeRedis`, `setHandshakeReplies`, `fake.connections()`. AUTH close-without-reply needs a listener that accepts, reads AUTH, and closes with no reply.
2. Four repros, `MaxRetries: 1`, assert accepts==1: AUTH close-no-reply; AUTH OK then SELECT close-no-reply (`Pass`+`Database`); AUTH `-LOADING Redis is loading the dataset in memory`; AUTH `-ERR max number of clients reached`.
3. THEN mark at `dial` return after AUTH/SELECT `do()`: keep inner `Error()`/`Unwrap()` (`redis:unreachable`, `LOADING …`, `redis:noauth`). `shouldRetry` returns false for that mark BEFORE `isUnreachable` / `isRetryableRedisReply`. Do not unmask inside `ioError`. TCP `DialContext` failure stays unmarked `errUnreachable` (still retries). GET LOADING still retries (`do(GET)` in `exec`, never marked).
4. THEN those four tests pass, plus existing WRONGPASS / SELECT 99 (1 conn) and GET LOADING retry.
5. Do not implement the mark until the failing tests exist in the worktree.

## Affected
- `simpleredis/pool.go` (`dial` AUTH/SELECT return)
- `simpleredis/commands_exec.go` (`shouldRetry`)
- `simpleredis/pool_test.go` (four repros; keep WRONGPASS / SELECT 99)
- `simpleredis/commands_exec_test.go` (GET LOADING still retries)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (handshake MUST NOT second TCP; scenarios for retryable handshake replies)

## Out of scope
- Eval deadlines
- Eval `$-1` miss
- Comments on MSetEX
- Other simpleredis bugs
- Changing `ioError` so EOF is not `errUnreachable`
- Unmasking handshake errors inside `do` / `ioError`

## Unknowns
- Wrapper / mark type name and file (not specified; agreed how is the mark, not the identifier).
- Whether AUTH/SELECT close-no-reply listeners stay as helpers in `pool_test.go` or a small extension of `fakeRedis`.

## Tensions
- Spec already forbids a second TCP connection; dest tests only cover non-retryable AUTH-class / SELECT 99, so retryable handshake replies still redial.
- AUTH stall with no reply is `redis:timeout` and is not retried (ticket control). AUTH close-no-reply is EOF → `errUnreachable` and is retried today — that is in scope.
- Ticket: do not invent another how. Mark at `dial` after AUTH/SELECT `do()`, not in `ioError`.
