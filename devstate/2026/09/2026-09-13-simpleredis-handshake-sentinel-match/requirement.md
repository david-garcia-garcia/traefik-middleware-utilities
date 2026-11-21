# Requirement
IssueKey: 2026-09-13-simpleredis-handshake-sentinel-match

## Problem
Under Yaegi, a SimpleRedis AUTH/SELECT handshake failure is an error no documented matcher can classify. Callers that follow `errors.Is` / `IsUnreachable` / `IsMiss` / `IsPoolWait` take the wrong branch on the outage paths those predicates exist to catch.

## Current (code)
- `simpleredis/pool.go` defines package-local `handshakeFailure` with `Error()` and `Unwrap()`, and `dial` returns `handshakeFailure{err: err}` on every AUTH/SELECT failure after TCP succeeds. TCP dial refuse returns bare `errUnreachable` (same file).
- `simpleredis/pool.go` `borrow` forwards that wrapped error unchanged. `simpleredis/commands_exec.go` `exec` returns the borrow error to the caller; `shouldRetry` short-circuits on `isHandshakeFailure` (type assert `err.(handshakeFailure)`) before `isUnreachable` (`err == errUnreachable`) and `isRetryableRedisReply` (prefix/`ERR max number of clients reached` on `Error()` text).
- `simpleredis/commands_exec.go` comment on `isHandshakeFailure`: type assert, not `errors.As`, because Yaegi panics on `As` for this struct.
- `simpleredis/simpleredis.go` package doc: match with `errors.Is` or `IsMiss` / `IsUnreachable` / `IsPoolWait`, not string equality. `IsUnreachable` is `errors.Is(err, ErrUnreachable)`. `ErrPoolWait` wraps `ErrUnreachable` so both have `Error()` text `redis:unreachable`.
- `simpleredis/resp.go` `replyError` maps AUTH-class prefixes to `errNoAuth`; other `-` replies are `errors.New(text)`. `ioError` maps non-deadline IO to `errUnreachable`.
- Compiled matching through the wrapper is pinned in `simpleredis/errors_test.go` for `%w` wraps, not for handshake. Existing handshake-no-redial tests in `simpleredis/pool_test.go` (`TestHandshakeAuthEOFMustNotOpenSecondConnection`, SELECT EOF, AUTH LOADING, AUTH max-clients, AUTH reject, SELECT reject) and `simpleredis/commands_exec_test.go` `TestShouldRetryHandshakeFailureIsFalse` assert type/retry/text/accept count. `TestLoadingReplyIsRetried` / `TestRetryableRedisRepliesAreRetried` still retry `LOADING ` and max-clients after a successful handshake.
- Yaegi suite `simpleredis/yaegi_test.go` happy paths plus `TestYaegi_MatchSentinels` / `MatchSentinels` which wrap `ErrMiss` with `fmt.Errorf("%w")`. Helpers `writeGopathSimpleredis` and `writeGopathFile` are in that file. `startAcceptFake` is in `simpleredis/pool_test.go`. `startFakeRedis`, `readCommand`, `statusOKReply`, `setHandshakeReplies` are in `simpleredis/fake_redis_test.go`.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` requires `errors.Is` against exported sentinels (including `ErrNoAuth`) and predicates; wrapped sentinels must still match. `openspec/specs/std_go_simpleredis_tcp-session/spec.md` handshake requirement: AUTH-class → `redis:noauth`; peer close / AUTH LOADING / AUTH max-clients MUST NOT open a second TCP connection; TCP refuse before handshake MAY retry; `LOADING ` after a successful handshake MAY retry. Handshake EOF scenarios currently require “an error”, not `IsUnreachable`.
- Yaegi vs compiled matcher matrix is recorded on `origin/bugfixes20260913:simpleredis/BUGS.md` section 1 and `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` (`TestBugInterpretedHandshakeFailureDefeatsUnreachableMatching`, `TestBugInterpretedHandshakeFailureDefeatsNoAuthMatching`, `//go:build bugrepro`). Those two files: **not found** on `origin/master`.

## Desired
- Stop carrying the do-not-retry mark in the error value’s type. `dial` returns `(*pooledConn, error, handshake bool)`; `borrow` forwards that bool; `exec` passes it to `shouldRetry` instead of sniffing the error type. Return the inner error unchanged so Yaegi callers see the same bare sentinel that already matches (`IsUnreachable` / `errors.Is(..., ErrNoAuth)`).
- Delete `handshakeFailure`, `isHandshakeFailure`, and `Unwrap`.
- Do not fix with `errors.As`. Do not tell callers to compare `Error()` text.
- Preserve: AUTH/SELECT EOF, AUTH `-LOADING ...`, AUTH `-ERR max number of clients reached` not retried (exactly 1 TCP accept). Still retried: TCP dial refusal before handshake; `LOADING ` after a successful handshake.
- Permanent untagged tests in the default suite: port both `TestBug*` functions (keep Yaegi assertion + compiled control); add interpreted matcher coverage for package error paths; keep existing handshake-no-second-connection tests green; rewrite `TestShouldRetryHandshakeFailureIsFalse` onto the new mechanism.
- Keep package constraints in `openspec/specs/std_go_simpleredis_tcp-session/spec.md`: stdlib-only source, no `unsafe`/cgo/generics, jitter `math/rand` `Int63n`, Yaegi workarounds intact, knobs frozen at `New`. Spec update belongs in later `openspec/` work so handshake errors match the exported-sentinel contract, including Yaegi.

## Affected
- Later implement (not this phase): `simpleredis/pool.go`, `simpleredis/commands_exec.go`, `simpleredis/yaegi_test.go`, `simpleredis/pool_test.go`, `simpleredis/commands_exec_test.go`, any new test file, `openspec/` (tcp-session and/or resp-commands).
- Verification later: `go vet ./simpleredis/`; `go test -count=1 -timeout 300s ./simpleredis/`.

## Out of scope
- Creating or editing `simpleredis/BUGS.md`. Touching `simpleredis/resp.go` or `simpleredis/simpleredis.go` in prepare.
- Other items on `origin/bugfixes20260913:simpleredis/BUGS.md` (lost in-use turn, desynced socket).
- Telling callers to match by `Error()` text. Fixing via `errors.As`. Changing retry of post-handshake `LOADING ` / max-clients. Importing go-redis/miniredis/vendor, `unsafe`, cgo, generics, or `math/rand/v2`. Exporting writable pool/timeout/retry knobs on `SimpleRedis`.
- `windowcounter` fail-open/closed branches (not asked).

## Unknowns
- Interpreted matcher failure was not re-run in this worktree (repro file is not on master). Mechanism and dest tests above are from this tree; the compiled-vs-Yaegi matrix is from `origin/bugfixes20260913` plus the caller spec.

## Tensions
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` handshake EOF scenarios require “an error” and 1 accept, not `IsUnreachable`. Matching through wrapping is required by `openspec/specs/std_go_simpleredis_resp-commands/spec.md` and `simpleredis/simpleredis.go`. Ticket wants both: no-redial stays, matchers work under Yaegi. Spec text for handshake matching is a later-phase update, not a product extra.
- Archived `openspec/changes/archive/2026-09-13-simpleredis-handshake-no-redial/` introduced the type wrap so `shouldRetry` could see the mark while `Unwrap` kept compiled `IsUnreachable`. That wrap is what Yaegi cannot unwrap.
- Returning the inner error without the out-of-band bool would retry AUTH LOADING and AUTH max-clients via `isRetryableRedisReply` (`simpleredis/commands_exec.go`). The bool is required to keep the existing no-redial contract.
