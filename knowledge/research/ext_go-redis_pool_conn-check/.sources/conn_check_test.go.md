---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check_test.go
title: go-redis connCheck tests (Unix)
fetched: 2026-09-13
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/pool/conn_check_test.go
---

Build same as Unix `conn_check.go`.

Open HTTP test conn: `connCheck` nil. After `conn.Close()`, `connCheck` errors.

`SetDeadline(time.Now())`, sleep 10ms, `connCheck` still nil — deadline reset inside `connCheck` is required for a healthy idle peek.

`isHealthyConn` on a wrapper that blocks in `Read` (and exposes `NetConn()`) stays healthy and does **not** read through the wrapper: pool health check must not unwrap buffered transports.

`checkForData` with `SetReadDeadline(time.Now().Add(-time.Second))` on an idle conn: no error, `hasData` false (CSC path clears the read deadline before peek).
