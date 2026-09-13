## 1. Failing Yaegi regressions (MUST exist untagged before the signature change)

- [ ] 1.1 Port `TestBugInterpretedHandshakeFailureDefeatsUnreachableMatching` and `TestBugInterpretedHandshakeFailureDefeatsNoAuthMatching` from `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` into `simpleredis/yaegi_test.go` (no `//go:build bugrepro`). Keep Yaegi assertion, compiled control, `writeGopathSimpleredis`, `writeGopathFile`, `startAcceptFake`, `startFakeRedis`, `setHandshakeReplies`
- [ ] 1.2 Run `go test -count=1 -timeout 120s -run TestBugInterpretedHandshakeFailure ./simpleredis/` and confirm both FAIL on dest (interpreted `false`, compiled controls pass)
- [ ] 1.3 Add interpreted matcher coverage in `simpleredis/yaegi_test.go` for package-returned handshake errors (not only `MatchSentinels` `%w` of `ErrMiss`)

## 2. Out-of-band handshakeFailed (only after 1.2)

- [ ] 2.1 Change `dial` to `(*pooledConn, error, handshakeFailed bool)`; AUTH/SELECT `do()` failure returns the inner error and `true`; TCP refuse returns `errUnreachable` and `false`; success returns `false`. Delete `handshakeFailure` and `Unwrap`
- [ ] 2.2 Forward `handshakeFailed` from `borrow`; `exec` passes it to `shouldRetry(err, handshakeFailed)` on the borrow-error path and `false` on the command `do` path. Delete `isHandshakeFailure`
- [ ] 2.3 Rewrite `TestShouldRetryHandshakeFailureIsFalse` onto `shouldRetry(err, handshakeFailed)`; update other `shouldRetry` call sites in `commands_exec_test.go` and `errors_test.go` (`false` for non-handshake)
- [ ] 2.4 Run `go vet ./simpleredis/` and `go test -count=1 -timeout 300s ./simpleredis/` until the ported tests pass and existing `TestHandshakeAuthEOFMustNotOpenSecondConnection`, `TestHandshakeSelectEOFMustNotOpenSecondConnection`, `TestHandshakeAuthLoadingMustNotOpenSecondConnection`, `TestHandshakeAuthMaxClientsMustNotOpenSecondConnection`, `TestHandshakeAuthRejectedMapsToNoAuthAndIsNotPooled`, `TestHandshakeSelectRejectedAfterAuthIsNotPooled`, and `TestLoadingReplyIsRetried` stay green

## 3. Specs

- [ ] 3.1 Confirm the `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands` deltas match the landed tests
- [ ] 3.2 Run `openspec validate --change simpleredis-handshake-sentinel-match --strict`
