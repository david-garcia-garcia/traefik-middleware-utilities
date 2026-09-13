## 1. Failing test first

- [ ] 1.1 Add `stalePooledSocketFake` in untagged `simpleredis/stale_pooled_socket_retry_test.go`: real TCP RESP, keep accepted sockets, drop them all, hold simultaneous Gets to warm idle. Reuse `readCommand`, `bulk`, `statusOKReply`, `pooledIdle`, `assertTurnsFullAndNoOverFrees`.
- [ ] 1.2 Add `TestStalePooledSocketSequentialGetsSucceedAfterPeerDrop`: PoolSize 8, default MaxRetries, backoff off; warm PoolSize idle with simultaneous Gets; drop every accepted socket; sequential Gets succeed; `OverFrees() == 0` and turns full. Confirm it fails on dest.
- [ ] 1.3 Add `TestStalePooledSocketMaxRetriesOffStillRecovers`: `MaxRetries: -1`, same drop, the next sequential Get succeeds. Confirm it fails on dest.

## 2. Force-dial after unused-socket EOF

- [ ] 2.1 In `simpleredis/pool.go`, keep package `borrow` as the three-value wrapper. Shared body takes `skipIdle`: when true, skip `takeIdleConn` and dial. Return whether the handed socket came from idle. Do not edit `takeIdleConn`.
- [ ] 2.2 In `simpleredis/commands_exec.go` `exec`, after `runOnConn` returns unused-socket `errUnreachable`, do not increment the attempt, set `skipIdle` for the rest of this loop, at most once. Handshake, timeout, pool wait, and closed-client paths stay as dest.
- [ ] 2.3 Confirm 1.2 and 1.3 now pass. Keep `TestPeerClosedIdleConnEOFIsRetried` green.

## 3. Invariants

- [ ] 3.1 `go vet ./simpleredis/`, `go build ./...`, `go test ./simpleredis/ -count=1`, `go test ./... -count=1 -short`.
- [ ] 3.2 Re-run `go test -tags simpleredis_bugs ./simpleredis/ -run 'TestBugDeadIdle' -v` from the caller checkout that has the tagged files.

## 4. Specs

- [ ] 4.1 Confirm the change delta matches the landed tests.
- [ ] 4.2 `openspec validate --change simpleredis-stale-pooled-socket-retry --strict`
