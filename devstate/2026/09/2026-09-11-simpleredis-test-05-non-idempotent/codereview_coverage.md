# Test coverage

Ticket job (caller + live `openspec/specs/std_go_simpleredis_tcp-session/spec.md`): every verb uses go-redis-shaped retry (`MaxRetries` / backoff sentinels, `shouldRetry` for `redis:unreachable` and named Redis `-ERR` replies, not `redis:timeout`); closed client must not spin extra retries; lost-reply INCR/INCRBY/EVAL may double-apply (fake + drop-relay Pester).

1. [hard] Critical path untested — `simpleredis/simpleredis.go:207-209` — `exec` returns immediately on `borrow` error when `sr.isClosed()` so a closed client does not run the `MaxRetries` loop (with backoff) on `redis:unreachable`; production: `if sr.isClosed() || !shouldRetry(err) { return nil, err }`; test: `TestCloseDrainsIdleAndDoesNotRepool` only asserts `Get` after `Close` is `redis:unreachable` and `fake.connections() == 1` (no dial), which stays green if `isClosed()` is removed and the loop retries unreachable borrow failures without opening sockets
   → After `Close`, assert a single attempt (e.g. borrow/dial counter or elapsed time bound with default `MaxRetries`) before `redis:unreachable`
   Status: done
   Argument: `TestClosedClientUnreachableIsNotRetried` bounds elapsed time below MinRetryBackoff after Close.

2. [hard] Edge case untested — `simpleredis/simpleredis.go:291-300` — `isRetryableRedisReply` special-cases `ERR max number of clients reached` and prefixes `READONLY `, `MASTERDOWN `, `CLUSTERDOWN ` (plus LOADING/TRYAGAIN already covered); production: `if text == "ERR max number of clients reached" { return true }` and `strings.HasPrefix(text, prefix)` for those prefixes; tests: `TestLoadingReplyIsRetried` (`-LOADING …`), `TestTryAgainReplyIsRetried` (`-TRYAGAIN …`); `(none)` for READONLY, MASTERDOWN, CLUSTERDOWN, or max-clients — reverting any of those branches leaves those tests green
   → `armErrorReplyOnceForTest` with `-READONLY …`, `-MASTERDOWN …`, `-CLUSTERDOWN …`, and `-ERR max number of clients reached`, then assert the follow-up command succeeds once
   Status: done
   Argument: `TestRetryableRedisRepliesAreRetried` arms READONLY, MASTERDOWN, CLUSTERDOWN, and max-clients once each.
