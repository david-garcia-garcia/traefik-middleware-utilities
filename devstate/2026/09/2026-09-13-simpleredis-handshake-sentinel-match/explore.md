# Explore
IssueKey: 2026-09-13-simpleredis-handshake-sentinel-match

## Concepts

SimpleRedis dials TCP, then AUTH/SELECT when `Config.Pass` / `Config.Database` are set. `dial` in `simpleredis/pool.go` wraps every AUTH/SELECT `do()` error in package-local `handshakeFailure` (`Error()` + `Unwrap()`). TCP refuse before AUTH returns bare `errUnreachable`. `borrow` forwards that error. `exec` in `simpleredis/commands_exec.go` asks `shouldRetry`, which type-asserts `handshakeFailure` (not `errors.As`: Yaegi panics `*target must implement error`) **before** identity `isUnreachable` and text `isRetryableRedisReply`.

Callers match with compiled `errors.Is` / `IsUnreachable`. Under Yaegi the wrapper is an interpreted struct; `errors.Is` does not see `Unwrap()`, so AUTH EOF (`redis:unreachable`) and AUTH WRONGPASS (`redis:noauth`) do not match. Type-assert `isHandshakeFailure` still works inside interpreted code, so no-redial (1 TCP accept) is intact.

Usage packet `knowledge/devdocs/std_go_simpleredis.md` already says match with `errors.Is` / `IsUnreachable` and do not retry handshake AUTH/SELECT failures. `MatchSentinels` in `simpleredis/yaegi_test.go` only wraps `ErrMiss` with compiled `fmt.Errorf("%w")` — the wrapper shape Yaegi can see.

```
TCP ok ── AUTH/SELECT fail ── handshakeFailure{inner} ── caller
                                      │
                                      ├ compiled errors.Is → inner sentinel  (works)
                                      └ Yaegi  errors.Is → stop at wrapper     (broken)
                                      └ type assert isHandshakeFailure         (works; 1 accept)
```

## Decisions

- Keep the no-redial contract. Do not put the mark in the error type. `dial` returns the inner error unchanged plus `handshakeFailed bool`; `borrow` forwards it; `exec` passes it into `shouldRetry` so AUTH/SELECT EOF, AUTH `-LOADING …`, and AUTH `-ERR max number of clients reached` stay not retried, while TCP refuse and post-handshake `LOADING ` still retry.
- Delete `handshakeFailure`, `isHandshakeFailure`, and `Unwrap`. Do not use `errors.As`. Do not match `Error()` text (ambiguous with `ErrPoolWait`).
- Port `TestBugInterpretedHandshakeFailureDefeatsUnreachableMatching` and `TestBugInterpretedHandshakeFailureDefeatsNoAuthMatching` from `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` into the default suite (untagged), keeping Yaegi + compiled control. Extend interpreted matcher coverage beyond `MatchSentinels`. Rewrite `TestShouldRetryHandshakeFailureIsFalse` onto `shouldRetry(err, handshakeFailed)`.
- Spec: update live `std_go_simpleredis_tcp-session` handshake THEN lines so peer-close is `IsUnreachable` and AUTH-class is `errors.Is(..., ErrNoAuth)`; add interpreted handshake-error matching to `std_go_simpleredis_resp-commands` (current Yaegi scenario is only a `%w` wrap of `ErrMiss`).
- Reproduced this worktree (`origin/master` + empty start): `go test -tags bugrepro -count=1 -timeout 120s -run TestBugInterpretedHandshakeFailure ./simpleredis/` — both tests FAIL; compiled controls in those tests passed (failure is the interpreted `false`). Throwaway `simpleredis/bugs_repro_test.go` deleted after the run.

## Open questions

- Q: Keep the asked triple return (`dial` / `borrow` return `(*pooledConn, error, handshakeFailed bool)`) rather than a result struct?
  Rank: bounded asked — Desired names the triple return; 2 production call sites of `shouldRetry` in `simpleredis/commands_exec.go` plus `dial` and `borrow` (1 each); test call sites in `simpleredis/commands_exec_test.go` and `simpleredis/errors_test.go`; all migratable here
  Decision: assumed — implement the asked triple return; name the bool `handshakeFailed` (true only after TCP succeeded and AUTH/SELECT failed). `shouldRetry(err, handshakeFailed)` returns false when `handshakeFailed` is true, else the existing table.
  By: explore

- Q: Which spec leaves get the matcher scenarios?
  Rank: bounded asked — Desired: spec update so handshake errors match the exported-sentinel contract, including Yaegi; Affected names tcp-session and/or resp-commands
  Decision: assumed — modify existing `std_go_simpleredis_tcp-session` (handshake EOF/LOADING/max-clients THEN `IsUnreachable` / text as today plus matcher) and `std_go_simpleredis_resp-commands` (interpreted `errors.Is` / `IsUnreachable` on handshake AUTH EOF and WRONGPASS, not only `%w` of `ErrMiss`). Propose confirms via FindSpecHost.
  By: explore

- Q: Where do the ported Yaegi tests live?
  Rank: additive asked — Desired requires both TestBug* in the default suite and more interpreted matcher coverage; helpers already live in `simpleredis/yaegi_test.go`
  Decision: assumed — add the Yaegi handshake matcher tests and extra package-error matcher coverage in `simpleredis/yaegi_test.go`; rewrite `TestShouldRetryHandshakeFailureIsFalse` in `simpleredis/commands_exec_test.go`; do not add `simpleredis/BUGS.md`.
  By: explore

- Q: Write a `knowledge/research/` packet on Yaegi `errors.Is` vs interpreted `Unwrap`?
  Rank: additive incidental — no criterion asks for a research folder; existing Traefik Yaegi packets cover generics, AfterFunc, unsafe, not this
  Decision: assumed — do not write; this worktree measured the two TestBug* failures; source comments on `isHandshakeFailure` already record the related `errors.As` panic. The fix removes the interpreted wrapper instead of depending on Yaegi exposing methods to compiled stdlib.
  By: explore
