## 1. Failing test first

- [x] 1.1 Add `peerDropAllFake` in untagged `simpleredis/peer_drop_all_test.go`: real TCP RESP, keep accepted sockets, drop them all, hold simultaneous Gets to warm idle. Reuse `readCommand`, `bulk`, `statusOKReply`, `pooledIdle`, `assertTurnsFullAndNoOverFrees`.
- [x] 1.2 Add `TestPeerDropAllSequentialGetsSucceedAfterPeerDrop`: PoolSize 8, default MaxRetries, backoff off; warm PoolSize idle with simultaneous Gets; drop every accepted socket; sequential Gets succeed; `OverFrees() == 0` and turns full. Confirm it fails on dest.

## 2. Force-dial after unused-socket EOF

- [x] 2.1 In `simpleredis/pool.go`, `borrowSocket` is the one borrow path. `skipIdle` skips `takeIdleConn` and dials. Return whether the handed socket came from idle. Tests that used three-value `borrow` call `borrowSocket(ctx, false)`. Do not edit `takeIdleConn`.
- [x] 2.2 In `simpleredis/commands_exec.go` `exec`, after `runOnConn` returns unused-socket `errUnreachable`, set `skipIdle` for the rest of this loop. MaxRetries still counts. Handshake, timeout, pool wait, and closed-client paths stay as dest.
- [x] 2.3 Confirm 1.2 now passes. Keep `TestPeerClosedIdleConnEOFIsRetried` and `TestLostReplyIncrMaxRetriesOff` green.

## 3. Invariants

- [x] 3.1 `go vet ./simpleredis/`, `go build ./...`, `go test ./simpleredis/ -count=1`, `go test ./... -count=1 -short`.
- [x] 3.2 Re-run `go test -tags simpleredis_bugs ./simpleredis/ -run 'TestBugDeadIdle' -v` from the caller checkout that has the tagged files.

## 4. Specs

- [x] 4.1 Confirm the change delta matches the landed tests.
- [x] 4.2 `openspec validate --change simpleredis-stale-pooled-socket-retry --strict`
