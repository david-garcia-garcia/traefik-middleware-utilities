# Standards

1. [hard] Name for the scope — `simpleredis/stale_pooled_socket_retry_test.go:15` — `stalePooledSocketFake` uses the package’s idle-timeout word (`takeIdleConn`’s `stale`) for a peer that drops young unused sockets dest still borrows
   → Rename to the peer job, matching `peerCloseFake` (`peerDropAllFake` or similar)
   Status: done
   Argument: renamed to `peerDropAllFake` in `peer_drop_all_test.go`; test `TestPeerDropAllSequentialGetsSucceedAfterPeerDrop`.
2. [hard] Leave a trail — `simpleredis/stale_pooled_socket_retry_test.go:47` — new `serve` has no job comment; `fakeRedis.serve` and `chaosFake.serve` each say which commands they answer
   → Add a succinct job comment (GET bulk plus hold for simultaneous warm, then write)
   Status: done
   Argument: added serve job comment on `peerDropAllFake.serve`.
