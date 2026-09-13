## 1. Failing compiled repros (MUST exist before the mark)

- [ ] 1.1 Add `pool_test.go` helpers that accept TCP, read one command, and close with no reply (AUTH close-no-reply; AUTH `+OK` then SELECT close-no-reply)
- [ ] 1.2 Add `TestHandshakeAuthEOFMustNotOpenSecondConnection` (`Pass`, `MaxRetries: 1`, MinRetryBackoff off); assert error and accepts==1
- [ ] 1.3 Add `TestHandshakeSelectEOFMustNotOpenSecondConnection` (`Pass`+`Database`, `MaxRetries: 1`, MinRetryBackoff off); assert error and accepts==1
- [ ] 1.4 Add `TestHandshakeAuthLoadingMustNotOpenSecondConnection` via `startFakeRedis` + `setHandshakeReplies` AUTH `-LOADING Redis is loading the dataset in memory`; assert that error text and accepts==1
- [ ] 1.5 Add `TestHandshakeAuthMaxClientsMustNotOpenSecondConnection` via `setHandshakeReplies` AUTH `-ERR max number of clients reached`; assert that error text and accepts==1
- [ ] 1.6 Run `go test -short ./simpleredis/` and confirm those four tests fail on dest (no `//go:build bugrepro`)

## 2. Dial mark (only after 1.6)

- [ ] 2.1 Add unexported `handshakeFailure` in `pool.go` (`Error()`/`Unwrap()` delegate to inner); wrap AUTH/SELECT `do()` errors at `dial` return; leave TCP `DialContext` unmarked
- [ ] 2.2 In `shouldRetry`, return false for `handshakeFailure` (`errors.As`) before `isUnreachable` / `isRetryableRedisReply`
- [ ] 2.3 Run `go test -short ./simpleredis/` until the four new tests pass and existing `TestHandshakeAuthRejectedMapsToNoAuthAndIsNotPooled`, `TestHandshakeSelectRejectedAfterAuthIsNotPooled`, and `TestLoadingReplyIsRetried` still pass

## 3. Specs

- [ ] 3.1 Confirm the `std_go_simpleredis_tcp-session` delta matches the landed tests
- [ ] 3.2 Run `openspec validate --change simpleredis-handshake-no-redial --strict`
