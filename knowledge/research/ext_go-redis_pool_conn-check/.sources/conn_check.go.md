---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/conn_check.go
title: go-redis Unix connCheck MSG_PEEK
fetched: 2026-09-13
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/pool/conn_check.go
---

Build: `linux || darwin || dragonfly || freebsd || netbsd || openbsd || solaris || illumos`.

`errUnexpectedRead` = `"unexpected read from socket"`.

`connCheck(conn net.Conn)`:
- `conn.SetDeadline(time.Time{})` first (“Reset previous timeout”).
- Type-assert outer conn to `syscall.Conn`. Not ok → `return nil`.
- Comment: do not unwrap buffered transports (`crypto/tls.Conn`); that can reveal an encrypted post-handshake record and, with the deadline cleared, wait forever in TLS. `isHealthyConn` would then `PeekReplyType` on the TLS stream.
- Else `checkSyscallConn(sysConn)`.

`checkSyscallConn`:
- `rawConn, err := sysConn.SyscallConn()`.
- `rawConn.Read(func(fd uintptr) bool { ...; return true })` — one shot, no wait.
- Inside: `n, _, err := syscall.Recvfrom(int(fd), buf[:1], syscall.MSG_PEEK|syscall.MSG_DONTWAIT)`.
- `n == 0 && err == nil` → `io.EOF`.
- `n > 0` → `errUnexpectedRead` (peek, does not consume).
- `err == syscall.EAGAIN || err == syscall.EWOULDBLOCK` → `nil`.
- else `sysErr = err`.

`underlyingSyscallConn` / `checkForData` / `maybeHasData` unwrap via `NetConn()` (TLS) and clear **read** deadline only (`SetReadDeadline(time.Time{})`). CSC / drainer, not the pool `Get` health path. `connCheck` does not call them.
