# Bug: Yaegi handshake AUTH/SELECT failure defeats documented sentinel matching

IssueKey: 2026-09-13-simpleredis-handshake-sentinel-match
issueHost: local
issueRef: none

## Summary

Under Yaegi (the actual Traefik plugin runtime) a handshake AUTH/SELECT failure becomes an error that NO documented matcher can classify.

## Root cause

`simpleredis/pool.go` defines `handshakeFailure`, a package-local struct with `Error()` and `Unwrap()`, and `dial` returns it on every AUTH/SELECT failure path. `simpleredis` runs INTERPRETED under Yaegi. `errors.Is` is the COMPILED stdlib function; it discovers unwrapping via `err.(interface{ Unwrap() error })`, an assertion Yaegi does not satisfy for an interpreted struct type. So `errors.Is` stops at the wrapper and never reaches the inner sentinel.

## Measured, same binary and fakes, compiled vs interpreted

- AUTH peer-close (inner `errUnreachable`): `Error()` == "redis:unreachable". compiled `IsUnreachable(err)` = true; interpreted = FALSE.
- AUTH `-WRONGPASS ...` (inner `errNoAuth`): `Error()` == "redis:noauth". compiled `errors.Is(err, ErrNoAuth)` = true; interpreted = FALSE.
- SELECT `-ERR DB index is out of range` and AUTH `-LOADING ...`: all matchers false interpreted.

Controls proving it is specific to this type (all correct interpreted): a bare sentinel from a dead port matches `IsUnreachable`; `fmt.Errorf("ctx: %w", ErrUnreachable)` matches; a miss matches `IsMiss`. Yaegi handles the COMPILED `*fmt.wrapError` fine. Only the package's own interpreted struct is opaque.

## Why it is a production killer

`handshakeFailure` wraps `errUnreachable` exactly when Redis accepts TCP but the handshake dies — restart, failover, `LOADING` during an RDB load, `ERR max number of clients reached`, a connection dropped mid-AUTH. Those are the moments a caller's fail-open vs fail-closed branch matters most, and a caller that follows the documented contract is the one that breaks.

## Spec violated

`simpleredis/simpleredis.go` package doc — "Exported sentinels. Match with errors.Is or IsMiss / IsUnreachable / IsPoolWait, not string equality." Plus `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (AUTH/SELECT peer close is redis:unreachable; AUTH-class prefixes map to redis:noauth).

## What is NOT broken

`isHandshakeFailure` uses a type assert, not `errors.As`, and Yaegi resolves an interpreted type assert inside interpreted code. Verified: interpreted AUTH EOF opens exactly 1 TCP accept, same as compiled. The handshake-no-redial behaviour WORKS and must keep working. Only the caller-facing matching contract is broken.

## Agreed fix direction

Stop carrying the "do not retry" mark in the error value's TYPE. `dial` already knows the failure is a handshake failure at the point it returns, so let it say so out of band and return the INNER error unchanged — then the caller sees the same bare sentinel that already matches under Yaegi.

- `dial` returns `(*pooledConn, error, handshake bool)`; `borrow` forwards that bool; `exec` passes it to `shouldRetry` instead of sniffing the error type.
- Delete `handshakeFailure`, `isHandshakeFailure`, and the `Unwrap`. Nothing then depends on Yaegi exposing an interpreted method to compiled stdlib code.
- Do NOT fix this with `errors.As` — the source comment records that Yaegi PANICS on `errors.As` for this struct, which is why the type assert exists.
- Do NOT tell callers to compare `Error()` text: the spec forbids string equality, and "redis:unreachable" is also `ErrPoolWait`'s text, so text is ambiguous.

## Behaviour you MUST preserve

`shouldRetry` currently short-circuits on `isHandshakeFailure` BEFORE `isUnreachable`/`isRetryableRedisReply`:

- NOT retried, exactly 1 TCP accept: AUTH/SELECT EOF, AUTH `-LOADING ...`, AUTH `-ERR max number of clients reached`.
- STILL retried: a TCP dial refusal before the handshake; a `LOADING ` reply AFTER a successful handshake.

## Tests that must land (permanent and untagged, running in the default suite)

Record as Desired, do not implement:

1. Port both `TestBug*` functions into permanent regression tests that PASS after the fix. They MUST keep the Yaegi interpreter assertion and the compiled control. Helpers: `writeGopathSimpleredis` and `writeGopathFile` in `simpleredis/yaegi_test.go`; `startAcceptFake` in `simpleredis/pool_test.go`; `startFakeRedis`, `readCommand`, `statusOKReply`, `setHandshakeReplies` in `simpleredis/fake_redis_test.go`.
2. Add interpreted matcher coverage for the error paths that return package errors (existing Yaegi suite covers only happy paths; `MatchSentinels` tests the wrapper shape that works).
3. Keep green the existing handshake-no-second-connection tests. `TestShouldRetryHandshakeFailureIsFalse` names the type — rewrite it to test the NEW mechanism.

Reference (read for analysis; do NOT copy BUGS.md into the product tree):

- `origin/bugfixes20260913:simpleredis/BUGS.md` section 1 is this bug
- `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` — build-tagged `//go:build bugrepro`; tests `TestBugInterpretedHandshakeFailureDefeatsUnreachableMatching` and `TestBugInterpretedHandshakeFailureDefeatsNoAuthMatching`

Neither file exists on master; later phases port the tests.

## Package constraints

From `openspec/specs/std_go_simpleredis_tcp-session/spec.md`:

- simpleredis source imports ONLY the Go standard library. No go-redis, no miniredis, no vendor pkgs.
- No `unsafe`, no cgo, no type parameters (generics).
- Retry jitter stays stdlib `math/rand` `Int63n`, NOT `math/rand/v2`.
- Keep Yaegi workarounds intact (errors.As panic, net.Error type asserts, context.AfterFunc in resp.go).
- Pool/timeout/retry knobs frozen at New on Config; SimpleRedis must not export writable ones.

## Owned files (later implement, not prepare)

`simpleredis/pool.go`, `simpleredis/commands_exec.go`, `simpleredis/yaegi_test.go`, `simpleredis/pool_test.go`, `simpleredis/commands_exec_test.go`, plus any new test file, plus `openspec/` for the spec update.

Do NOT create or edit `simpleredis/BUGS.md`.
Do NOT touch `simpleredis/resp.go` or `simpleredis/simpleredis.go` in prepare.

## Verification gates (record, do not run the whole suite unless a fact is needed)

- go vet ./simpleredis/
- go test -count=1 -timeout 300s ./simpleredis/
