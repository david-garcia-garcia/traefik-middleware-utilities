## Context

Dest `dial` wraps AUTH/SELECT `do()` errors in unexported `handshakeFailure` (`Error()` + `Unwrap()`). `shouldRetry` type-asserts that type before identity `isUnreachable` and text `isRetryableRedisReply`. Compiled `errors.Is` unwraps it; Yaegi does not, so interpreted `IsUnreachable` / `errors.Is(..., ErrNoAuth)` are false while `Error()` is the sentinel text. Type-assert `isHandshakeFailure` still works interpreted, so no-redial (1 TCP accept) is intact. See proposal.md. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Return the inner handshake error unchanged so Yaegi callers match the same bare sentinels compiled callers already match.
- Keep AUTH/SELECT EOF, AUTH `-LOADING …`, and AUTH `-ERR max number of clients reached` at exactly one TCP accept.
- Keep TCP dial refuse and post-handshake `LOADING ` retried.
- Permanent untagged Yaegi+compiled regressions in the default suite.

**Non-Goals:**
- Fixing via `errors.As` (Yaegi panics `*target must implement error` on this struct).
- Matching `Error()` text (`redis:unreachable` is also `ErrPoolWait`).
- Creating or editing `simpleredis/BUGS.md`.
- Other items on `origin/bugfixes20260913:simpleredis/BUGS.md` (lost in-use turn, desynced socket).
- Changing `ioError`, `isUnreachable` identity, `math/rand/v2`, generics, `unsafe`, or exported writable knobs.

## Decisions

1. **Out-of-band `handshakeFailed` bool, not an error type.** `dial` returns `(*pooledConn, error, handshakeFailed bool)`; `borrow` forwards that bool; `exec` passes it to `shouldRetry`. AUTH/SELECT `do()` failure after TCP: return the inner error and `handshakeFailed true`. TCP `DialContext` refuse: inner `errUnreachable` and `handshakeFailed false`. Alternative: keep `handshakeFailure` and teach callers `Error()` — rejected; spec forbids string equality and the text is ambiguous with pool wait. Alternative: `errors.As` — rejected; Yaegi panics. Alternative: `fmt.Errorf("%w")` — rejected; identity `isUnreachable` would miss, and LOADING `Error()` would change unless the wrapper delegates, which is the current opaque type.

2. **Name the bool `handshakeFailed`.** The ask said `handshake bool`. A bare `handshake` flag does not name the AUTH/SELECT-failed cases `shouldRetry` cares about. Deviation recorded on `devstate/deviations.md`.

3. **`shouldRetry(err, handshakeFailed)` returns false when `handshakeFailed` is true**, else the existing table (`isCommandTimeout`, identity unreachable, retryable Redis reply text). Two call sites in `exec` (borrow error and command `do` error). Command `do` errors are never handshake; pass `false`. Delete `handshakeFailure` / `isHandshakeFailure`. Rewrite `TestShouldRetryHandshakeFailureIsFalse` to pin `shouldRetry(errUnreachable, true) == false`, `shouldRetry(LOADING, true) == false`, `shouldRetry(errUnreachable, false) == true`, `shouldRetry(LOADING, false) == true`.

4. **Port both `TestBug*` into `simpleredis/yaegi_test.go`.** Keep Yaegi assertion and compiled control. Reuse `writeGopathSimpleredis`, `writeGopathFile`, `startAcceptFake`, `startFakeRedis`, `setHandshakeReplies`. Extend interpreted matcher coverage for package-returned errors (handshake AUTH EOF and WRONGPASS). Do not add `BUGS.md`.

5. **Fold into existing leaves.** FindSpecHost: small adjustment to `std_go_simpleredis_tcp-session` (handshake THEN matchers) and `std_go_simpleredis_resp-commands` (interpreted handshake-error matching). No new spec leaf.

## Risks / Trade-offs

- [Returning bare `errUnreachable` from AUTH EOF would retry unless the bool is threaded] → Mitigation: `shouldRetry` short-circuits on `handshakeFailed` before `isUnreachable` / `isRetryableRedisReply`; existing 1-accept tests stay.
- [Returning AUTH LOADING / max-clients text would retry via `isRetryableRedisReply`] → Mitigation: same `handshakeFailed` short-circuit.
- [Yaegi tests copy non-test sources into GOPATH] → Mitigation: the production change is in those copied files; the interpreter assertion is the proof.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy keys. Callers that already used `errors.Is` / `IsUnreachable` start working under Yaegi; compiled callers keep matching.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
