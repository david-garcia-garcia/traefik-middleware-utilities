---
url: https://github.com/golang/go/blob/go1.25.6/src/crypto/tls/conn.go
title: crypto/tls Conn remote alerts and Close (Go 1.25.6)
fetched: 2026-11-21
authority: source
ref: golang/go@go1.25.6:src/crypto/tls/conn.go
---

Received alert (lines ~737, ~744): `return c.in.setErrorLocked(&net.OpError{Op: "remote error", Err: alert(data[1])})` — `alert` is unexported; `AlertError` (alert.go) is documented for QUIC: "When using a QUIC transport, QUICConn methods will return an error which wraps AlertError rather than sending a TLS alert."

`(*Conn).Close`: when no Write is in flight and the handshake is complete, calls `closeNotify()`, which does `c.SetWriteDeadline(time.Now().Add(time.Second * 5))`, sends `alertCloseNotify`, then `SetWriteDeadline(time.Now())`, then closes the raw conn. When a Write is in flight it closes the raw conn without the alert.
