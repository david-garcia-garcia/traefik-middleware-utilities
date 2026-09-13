# Explore
IssueKey: 2026-09-13-simpleredis-bug-handshake-redial

## Concepts

Handshake AUTH/SELECT runs inside `dial` after TCP `DialContext`. Failures return the `do` error unchanged. `exec` retries when `shouldRetry` is true: identity `errUnreachable` (EOF via `ioError`) and Redis texts `LOADING ` / exact `ERR max number of clients reached`. Spec `std_go_simpleredis_tcp-session` “Handshake AUTH or SELECT failure” already requires one error and MUST NOT open a second TCP connection. Dest tests cover WRONGPASS/NOAUTH (not retryable) and SELECT 99, so retryable handshake replies still redial.

Agreed how (ticket; do not invent another): mark at `dial` return after AUTH/SELECT `do()`; keep inner `Error()`/`Unwrap()`; `shouldRetry` returns false for that mark before `isUnreachable` / `isRetryableRedisReply`. Do not unmask inside `ioError` (shared with GET lost-reply). TCP `DialContext` refuse stays unmarked `errUnreachable` (still retries). GET LOADING stays unmarked (still retries).

```
Get → exec → borrow → dial
                         ├ DialContext fail → unmarked errUnreachable → retry
                         ├ AUTH/SELECT do() fail → mark handshakeFailure → no retry
                         └ ok → command do(GET)
                                      └ LOADING / EOF → unmarked → retry
```

## Decisions

- Tests first: four compiled repros in `simpleredis/pool_test.go` under default `go test -short ./simpleredis/`. They MUST fail on dest, then the mark lands, then they pass plus WRONGPASS / SELECT 99 / GET LOADING.
- Mark type: unexported wrapper produced by `dial`, consumed by `shouldRetry`. Same wrap idea as `ErrPoolWait` (identity ≠ `errUnreachable`) but a named type so `Error()` stays the inner text (`LOADING …` still matches `isRetryableRedisReply` unless the mark is checked first).
- AUTH/SELECT close-no-reply: dedicated accept-and-close listeners in `pool_test.go`. Do not reuse `armCloseBeforeReplyOnceForTest` (that is the GET lost-reply path). LOADING and max-clients: `startFakeRedis` + `setHandshakeReplies`.
- Spec: fold scenarios onto existing `std_go_simpleredis_tcp-session` handshake requirement. OpenSpec change name `simpleredis-handshake-no-redial`.
- No identity reconstruction (no client address / user / tenant / Host / trust hop).
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` already tells callers to match `Error()` / `errors.Is` on sentinels; handshake mark stays unexported so that contract holds.

## Open questions

- Q: What is the handshake mark type name and file?
  Rank: additive asked — new type this change creates; Desired item 3 names the mark at dial return after AUTH/SELECT do()
  Decision: assumed — unexported `handshakeFailure` in `simpleredis/pool.go` next to `dial` (DTO that exists only to feed `shouldRetry`). `Error()` and `Unwrap()` delegate to inner. `shouldRetry` in `commands_exec.go` uses `errors.As` and returns false before `isUnreachable` / `isRetryableRedisReply`. Two call sites of `shouldRetry` (borrow error and command `do` error in `exec`).
  By: explore

- Q: Where do AUTH/SELECT close-without-reply listeners live?
  Rank: additive asked — test helpers this change creates; Desired item 1 prefers pool_test.go
  Decision: assumed — `startAcceptFake`-style helpers in `simpleredis/pool_test.go` for AUTH close-no-reply and AUTH OK then SELECT close-no-reply. AUTH `-LOADING …` and AUTH `-ERR max number of clients reached` reuse `startFakeRedis` + `setHandshakeReplies`. Do not extend `fakeRedis` for close-without-reply (`armCloseBeforeReplyOnceForTest` stays GET-only).
  By: explore

- Q: OpenSpec change name and spec host?
  Rank: additive asked — new change folder this run creates; Affected names std_go_simpleredis_tcp-session
  Decision: assumed — change `simpleredis-handshake-no-redial`. FindSpecHost fold into existing `std_go_simpleredis_tcp-session`. Add scenarios for AUTH EOF, SELECT EOF, AUTH LOADING, AUTH max-clients (accepts==1). Do not create a new spec leaf.
  By: explore

- Q: Should the handshake mark be exported?
  Rank: additive incidental — new type this change creates; requirement does not name a public API
  Decision: assumed — keep unexported. Callers keep matching inner `Error()` / `Unwrap()` (`redis:unreachable`, `LOADING …`, `redis:noauth`).
  By: explore

- Q: Should `isUnreachable` switch from identity `==` to `errors.Is`?
  Rank: bounded incidental — `isUnreachable` has 1 call site in `shouldRetry` (`simpleredis/commands_exec.go`); requirement Out of scope says do not change ioError / unmask
  Decision: assumed — leave `isUnreachable` as `err == errUnreachable`. Handshake gate is the mark check. Changing identity match would be a second job.
  By: explore
