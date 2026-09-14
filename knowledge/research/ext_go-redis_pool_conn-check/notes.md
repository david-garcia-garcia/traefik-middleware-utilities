# Idle conn unread-data check

How `github.com/redis/go-redis` detects unread bytes on a parked TCP socket before reuse. Pin: [redis/go-redis@7f3b3df](https://github.com/redis/go-redis/tree/7f3b3dffde59329db9fa7a71d650ef373affb599) (`v9.23.0-beta.1`). SimpleRedis does **not** import go-redis. This folder is the outside-system fact: Unix peeks the kernel receive queue; Windows and other non-unix builds skip the peek.

See `ext_go_net_setreaddeadline/` for the stdlib `SetReadDeadline` / zero-`Time` contract. go-redis Unix `connCheck` uses that API only to **clear** a leftover deadline before the peek.

## Get pops idle, then isHealthyConn

After `popIdle`, `Get` calls `isHealthyConn`. Lifetime and idle-time fail first (no syscall). If those pass, it runs `connCheck(cn.getNetConn())`. Any error other than the RESP3 push exception below closes the socket as stale and retries another idle conn. ([go-redis@7f3b3dff:internal/pool/pool.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/pool.go) `getConn` / `isHealthyConn`, [.sources/pool.go.md](.sources/pool.go.md))

## Unix connCheck peeks one byte and does not consume it

Build constraint: `linux || darwin || dragonfly || freebsd || netbsd || openbsd || solaris || illumos`. ([go-redis@7f3b3dff:internal/pool/conn_check.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check.go), [.sources/conn_check.go.md](.sources/conn_check.go.md))

`connCheck`:

1. `conn.SetDeadline(time.Time{})` — clear a leftover I/O deadline so the peek is not a timeout. Test: `SetDeadline(time.Now())`, sleep 10ms, `connCheck` still succeeds. ([conn_check.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check.go); [conn_check_test.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check_test.go), [.sources/conn_check_test.go.md](.sources/conn_check_test.go.md))
2. Type-assert the **outer** `net.Conn` to `syscall.Conn`. If that fails, return `nil` (treat as no unread-data signal). Comment: do not unwrap `crypto/tls.Conn`; an inner peek can surface a post-handshake record and, with the deadline cleared, block in TLS. ([conn_check.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check.go), [.sources/conn_check.go.md](.sources/conn_check.go.md))
3. `SyscallConn()`, then `rawConn.Read` with a one-shot callback that always returns `true` (no wait). Inside: `syscall.Recvfrom(int(fd), buf[:1], syscall.MSG_PEEK|syscall.MSG_DONTWAIT)`. Those two flags and `Recvfrom` are what this pin calls; this folder does not invent other peek APIs. ([conn_check.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check.go) `checkSyscallConn`, [.sources/conn_check.go.md](.sources/conn_check.go.md))

Result of that `Recvfrom`:

| result | `connCheck` error |
|--------|-------------------|
| `n == 0 && err == nil` | `io.EOF` (peer closed) |
| `n > 0` | `errUnexpectedRead` (`"unexpected read from socket"`) — kernel queue has unread bytes; peek does not consume them |
| `err == syscall.EAGAIN` or `syscall.EWOULDBLOCK` | `nil` (idle, no byte ready) |
| any other `err` | that error |

Closed client socket: `connCheck` errors. ([conn_check_test.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check_test.go), [.sources/conn_check_test.go.md](.sources/conn_check_test.go.md))

Because the peek does not consume, `isHealthyConn` can still pull the type byte into the proto reader via `PeekReplyTypeForCheck` when unexpected data might be a RESP3 push. ([go-redis@7f3b3dff:internal/pool/conn.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn.go) `PeekReplyTypeForCheck`, [.sources/conn.go.md](.sources/conn.go.md))

## Unexpected data is unhealthy unless it is a RESP3 push

`errUnexpectedRead` with `PushNotificationsEnabled` and `PeekReplyTypeForCheck` returning `proto.RespPush` → still healthy (client will drain pushes). Any other unexpected byte, EOF, or syscall error → not healthy. ([pool.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/pool.go) `isHealthyConn`, [.sources/pool.go.md](.sources/pool.go.md))

## Windows and other non-unix: connCheck is a no-op

Build constraint: `!linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !solaris && !illumos` (Windows included). `connCheck` ignores the `net.Conn` and **always returns `nil`**. Comment on this pin: there is no portable non-consuming readiness check on this platform. An idle Windows socket with unread kernel bytes still passes `isHealthyConn` if lifetime/idle-time pass. ([go-redis@7f3b3dff:internal/pool/conn_check_dummy.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check_dummy.go), [.sources/conn_check_dummy.go.md](.sources/conn_check_dummy.go.md))

`maybeHasData` / `checkForData` on the dummy also report no data. Those helpers are the CSC drainer path (Unix `checkForData` *does* unwrap via `NetConn()`). Idle-pool `Get` uses `connCheck`, not `maybeHasData`.
