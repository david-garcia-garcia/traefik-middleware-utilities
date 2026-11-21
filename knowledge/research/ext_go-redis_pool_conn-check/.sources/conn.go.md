---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn.go
title: go-redis PeekReplyTypeForCheck after MSG_PEEK
fetched: 2026-09-13
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/pool/conn.go
---

`getNetConn()` returns the atomic `net.Conn` (or nil). `isHealthyConn` passes that to `connCheck`.

`PeekReplyTypeForCheck` holds `readerMu` and calls `cn.rd.PeekReplyType()`. Comment: unlike `PeekReplyTypeSafe` it does not require data already buffered in the proto reader. Pool health check calls it after `connCheck` reports unexpected socket data; `connCheck` only MSG_PEEKs, so the type byte still has to be pulled from the socket into the reader here.
