## Context

Dest `dial` runs AUTH then SELECT after TCP `DialContext`. Handshake `do` errors return unmarked. `exec` retries `errUnreachable` (EOF via `ioError`) and Redis texts `LOADING ` / exact `ERR max number of clients reached`. `ioError` is shared with GET; unmasking there would break lost-reply retry and `Error()` `redis:unreachable`. See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Four compiled repros that fail on dest, then a dial-return mark so handshake AUTH/SELECT failures are not retried.
- Preserve TCP refuse retry and GET LOADING retry.
- Keep inner `Error()` / `Unwrap()` so callers still match `redis:unreachable`, `LOADING …`, and `redis:noauth`.

**Non-Goals:**
- Changing `ioError`.
- Exporting the mark.
- Switching `isUnreachable` from identity `==` to `errors.Is`.
- Eval deadlines, Eval `$-1` miss, MSetEX comments, other simpleredis bugs.

## Decisions

1. **Tests land before the mark.** Four default `go test -short ./simpleredis/` cases in `pool_test.go` MUST fail on dest, then the mark is applied, then they pass plus WRONGPASS / SELECT 99 / GET LOADING. Alternative: implement first — rejected; the ticket requires failing tests exist before the fix.

2. **Unexported `handshakeFailure` in `pool.go` next to `dial`.** `Error()` and `Unwrap()` delegate to inner. `shouldRetry` uses `errors.As` and returns false before `isUnreachable` / `isRetryableRedisReply`. Two `shouldRetry` call sites in `exec` (borrow error and command `do` error). Alternative: unmask inside `ioError` — rejected; shared with GET. Alternative: `fmt.Errorf` wrap like `ErrPoolWait` — rejected; that would change `LOADING …` `Error()` text unless the wrapper delegates, which is this named type.

3. **Mark only AUTH/SELECT `do()` returns.** TCP `DialContext` failure stays unmarked `errUnreachable` (still retries). GET LOADING is `do(GET)` in `exec`, never marked (still retries). AUTH stall with no reply stays `redis:timeout` (already not retried).

4. **Close-no-reply helpers live in `pool_test.go`.** AUTH close-no-reply and AUTH OK then SELECT close-no-reply use a listener that accepts, reads, and closes with no reply. AUTH LOADING and AUTH max-clients reuse `startFakeRedis` + `setHandshakeReplies`. Do not extend `armCloseBeforeReplyOnceForTest` (GET lost-reply path).

5. **Fold into `std_go_simpleredis_tcp-session`.** FindSpecHost: small adjustment to the existing handshake requirement. Add four scenarios. Do not create a new spec leaf.

## Risks / Trade-offs

- [Mark wrapping `errUnreachable` still matches exported `IsUnreachable` via `Unwrap`] → Mitigation: keep Unwrap; callers matching sentinels stay; `shouldRetry` gates on the mark first so retry does not.
- [LOADING handshake `Error()` still matches `isRetryableRedisReply`] → Mitigation: mark check runs before that predicate.
- [Close-no-reply listener races Accept vs Get] → Mitigation: same accept-count pattern as existing fakes; assert after Get returns.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy keys.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
