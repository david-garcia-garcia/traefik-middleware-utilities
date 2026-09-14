# Conn SetReadDeadline

Official Go `net.Conn` read-deadline contract. Not go-redis. This folder is for a one-byte idle probe that uses only stdlib `net.Conn` (Yaegi-safe): zero `Time` vs an already-exceeded deadline.

## Zero Time means no read timeout

`SetReadDeadline(t)` sets the deadline for future Read calls and any currently-blocked Read. **A zero value for `t` means Read will not time out.** `SetDeadline` is both read and write; a zero `t` there means I/O operations will not time out. ([net.Conn](https://pkg.go.dev/net#Conn), [.sources/conn.md](.sources/conn.md))

`SetReadDeadline(time.Time{})` therefore **clears** the timeout. A following `Read` waits for data (or EOF/error). It is not a non-blocking poll.

## An already-exceeded deadline fails a Read that would block

A deadline is an **absolute** time after which I/O operations fail instead of blocking. It applies to all future and pending I/O, not only the next Read. If the deadline is exceeded, Read returns an error that wraps `os.ErrDeadlineExceeded` (`"i/o timeout"`). `errors.Is(err, os.ErrDeadlineExceeded)` is the test. That error’s `Timeout()` method returns true; other errors may also report `Timeout() == true`. ([net.Conn](https://pkg.go.dev/net#Conn), [.sources/conn.md](.sources/conn.md); [os.ErrDeadlineExceeded](https://pkg.go.dev/os#ErrDeadlineExceeded), [.sources/err-deadline-exceeded.md](.sources/err-deadline-exceeded.md))

Setting `t` to `time.Now()` (or any time that is not in the future) is that exceeded case: a Read that would have to wait for the peer fails with the timeout instead of blocking. pkg.go.dev does not name `time.Now()` as a probe API; that mapping is from the absolute-time rule. ([net.Conn](https://pkg.go.dev/net#Conn); authority: inference from the official deadline contract)

After the deadline has been exceeded, later Reads keep failing until the connection is refreshed by setting a deadline in the future (or zero). Restore the intended timeout after a probe. ([net.Conn](https://pkg.go.dev/net#Conn), [.sources/conn.md](.sources/conn.md))
