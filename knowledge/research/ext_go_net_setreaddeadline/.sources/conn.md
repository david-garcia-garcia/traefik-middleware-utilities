---
url: https://pkg.go.dev/net#Conn
title: net.Conn SetReadDeadline
fetched: 2026-09-13
authority: official
---

`Conn` is a generic stream-oriented network connection.

`SetDeadline(t time.Time) error` is equivalent to `SetReadDeadline` and `SetWriteDeadline`.

A deadline is an absolute time after which I/O operations fail instead of blocking. The deadline applies to all future and pending I/O, not just the immediately following call to Read or Write. After a deadline has been exceeded, the connection can be refreshed by setting a deadline in the future.

If the deadline is exceeded a call to Read or Write or to other I/O methods will return an error that wraps `os.ErrDeadlineExceeded`. Test with `errors.Is(err, os.ErrDeadlineExceeded)`. The error’s `Timeout` method will return true; other errors may also have `Timeout() == true` even if the deadline has not been exceeded.

A zero value for `t` means I/O operations will not time out (`SetDeadline`).

`SetReadDeadline(t time.Time) error` sets the deadline for future Read calls and any currently-blocked Read call. A zero value for `t` means Read will not time out.

`SetWriteDeadline`: a zero value for `t` means Write will not time out.
