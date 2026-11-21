# crypto/tls client dial

What Go's `crypto/tls` client does when a caller dials with `tls.Dialer{NetDialer, Config}.DialContext`: which timeout covers the handshake, where `ServerName` comes from, how a peer's alert is returned, and what `Close` sends.

## The NetDialer timeout covers TCP plus handshake

`dial` wraps ctx with `NetDialer.Timeout` before the TCP connect and runs `HandshakeContext(ctx)` on that same ctx, so one `Timeout` bounds connect and handshake together. `DialWithDialer` documents the same ("apply to connection and TLS handshake as a whole") but uses `context.Background` internally; `Dialer.DialContext` is the form that also honours the caller's ctx. Once the handshake completes, ctx expiry does not affect the connection.

Source: `golang/go@go1.25.6:src/crypto/tls/tls.go` (`dial`, `(*Dialer).DialContext`), `.sources/tls.go.md`; https://pkg.go.dev/crypto/tls `DialWithDialer`, `.sources/pkg-go-dev-crypto-tls.md`.

## Empty ServerName is inferred from the dial address

When `Config.ServerName == ""`, `dial` clones the config and sets `ServerName` to the host part of `addr` (text before the last `:`). Verification then checks that name, or IP SANs when the host is an IP literal. The caller's config is not modified. `ServerName` is used to verify the hostname unless `InsecureSkipVerify`.

Source: `.sources/tls.go.md`; `.sources/pkg-go-dev-crypto-tls.md`. Measured under Yaegi: `ext_traefik_plugins_yaegi-tls`.

## Clone is shallow and concurrency-safe

"Clone returns a shallow clone of c or nil if c is nil. It is safe to clone a Config that is being used concurrently by a TLS client or server." Shallow means `RootCAs`, `Certificates`, and callback fields are shared with the original.

Source: https://pkg.go.dev/crypto/tls `Config.Clone`, `.sources/pkg-go-dev-crypto-tls.md`.

## A peer alert is net.OpError "remote error", not AlertError

A TLS alert received from the peer is returned as `&net.OpError{Op: "remote error", Err: alert(n)}` where `alert` is unexported. `tls.AlertError` is the QUIC-path type and does not match it. Error text is `remote error: tls: <alert>` (for example `handshake failure`, `certificate required`). A failed server-certificate check on the client is `*tls.CertificateVerificationError`.

Source: `golang/go@go1.25.6:src/crypto/tls/conn.go`, `alert.go`, `.sources/conn.go.md`; https://pkg.go.dev/crypto/tls `CertificateVerificationError`.

## Close sends close_notify with a 5s write deadline

`(*Conn).Close` after a completed handshake sends a `close_notify` alert under a 5-second write deadline, then closes the raw conn. If a `Write` is in flight it skips the alert and closes the raw conn.

Source: `.sources/conn.go.md`.

## Defaults

`Config.MinVersion` defaults to TLS 1.2. A nil `Config` on `tls.Dial` is the zero configuration.

Source: `.sources/pkg-go-dev-crypto-tls.md`.
