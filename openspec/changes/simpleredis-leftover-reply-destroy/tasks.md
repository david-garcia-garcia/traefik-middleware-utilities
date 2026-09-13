## 1. Failing tests first

- [x] 1.1 Port `startStrayExtraReplyFake` from `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` into `simpleredis/fake_redis_test.go` (wire `$5\r\nSTRAY\r\n`). Untagged.
- [x] 1.2 Add `TestDesyncedSocketDoesNotServePreviousReplies` in `simpleredis/resp_test.go`: `PoolSize` 1, `MaxRetries` -1, Get `k0`..`k11` against nth=5 stray fake; each Get is own-key or error, never another key or `STRAY` as success. Confirm it fails on dest.
- [x] 1.3 Add `TestStrayExtraReplyIsNotPooled`: after the stray command, `pooledIdle == 0` and the next Get increments the fake accept count. Confirm it fails on dest.
- [x] 1.4 Keep `TestConnectionIsReused` (25 Gets → exactly 1 accept). Do not weaken the count.

## 2. Gate in `do`

- [x] 2.1 In `simpleredis/resp.go` `do`, after a successful `readReply`, set `reusable` true only when `conn.reader.Buffered() == 0`. Leftover: return the decoded slots and `reusable = false`. Do not drain.
- [x] 2.2 Before `writeCommand`, if `Buffered() != 0`, do not write; return `reusable = false` and `redis:issue?`. Do not edit `pool.go` or `commands_exec.go`.
- [x] 2.3 Confirm 1.2 and 1.3 now pass. Confirm `TestConnectionIsReused` still reports 1 connection.

## 3. Invariants and Yaegi

- [x] 3.1 Keep green: `TestLoadingReplyIsRetried`, `TestTryAgainReplyIsRetried`, `TestRetryableRedisRepliesAreRetried`, `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestPeerClosedIdleConnEOFIsRetried`, `TestStaleIdleHeadIsClosedWhileTailStaysHot`, `TestTruncatedBulkIsUnreachableAndNotPooled`, `TestAuthAndSelectOncePerDial`, MGET / Eval / MSetEX array-decode tests.
- [x] 3.2 If `clientprobeSrc` can express Get against the stray fake without a second interpreter harness, add that Yaegi test. Otherwise skip.
- [x] 3.3 `go vet ./simpleredis/` and `go test -count=1 -timeout 300s ./simpleredis/`. Record the `TestConnectionIsReused` accept count.

## 4. Specs

- [x] 4.1 Confirm the change deltas match the landed tests.
- [x] 4.2 `openspec validate --change simpleredis-leftover-reply-destroy --strict`
