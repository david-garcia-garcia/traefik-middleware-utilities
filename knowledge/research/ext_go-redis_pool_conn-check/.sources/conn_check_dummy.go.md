---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check_dummy.go
title: go-redis non-unix connCheck no-op
fetched: 2026-09-13
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/pool/conn_check_dummy.go
---

Build: `!linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !solaris && !illumos` (Windows and other non-unix).

`errUnexpectedRead` still defined as `"unexpected read from socket"` (placeholder).

`connCheck(_ net.Conn) error` always `return nil`.

`maybeHasData` always `false`. `checkForData` returns `(false, nil)`.

Comment on `maybeHasData`: “There is no portable non-consuming readiness check on this platform. Returning true would force every idle CSC connection through a timed read on every drainer tick.”
