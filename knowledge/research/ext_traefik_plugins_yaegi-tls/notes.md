# Yaegi crypto/tls client

Whether interpreted code under Yaegi v0.16.1 with `stdlib.Symbols` only (Traefik with `useUnsafe` false, this module's `go.mod` pin) can run a TLS client the way SimpleRedis dials: `tls.Dialer{NetDialer: &net.Dialer{Timeout}, Config}.DialContext`, a `net.Conn` field, `bufio`, `SetDeadline`, `errors.Is` / `errors.As`.

## Symbols are present

Yaegi v0.16.1 extracts `crypto/tls` and `crypto/x509` for Go 1.21 and Go 1.22+ builds. `Symbols["crypto/tls/tls"]` maps `Dialer`, `Config`, `X509KeyPair`, `VersionTLS12`, `CertificateVerificationError`, `AlertError`, `RecordHeaderError`, `Client`, `Dial`, `DialWithDialer`. Methods such as `(*Config).Clone` are reached through the compiled type.

Source: `github.com/traefik/yaegi@v0.16.1:stdlib/go1_22_crypto_tls.go`; extract `.sources/go1_22_crypto_tls.go.md`.

## Measured behaviour (interpreted client)

Throwaway probe, not product code: compiled host on `go1.25.6 windows/amd64`, `interp.New(Options{GoPath})`, `Use(stdlib.Symbols)` only, interpreted package dialing compiled in-process TLS fakes (ECDSA P-256, own CA) and live `redis:7-alpine` 7.4.10 / Dragonfly v1.40.2 containers. Measured 2026-11-21. Extract `.sources/yaegi-tls-explore-probe.md`.

- `(*tls.Config).Clone()` runs. The clone is a different pointer; writing `RootCAs`, `ServerName`, and `VerifyPeerCertificate` on the original after the clone does not change the clone (the clone still verifies and dials; the mutated original fails verification). `Clone()` of a nil `*tls.Config` returns nil. An interpreted `VerifyPeerCertificate` closure survives the clone and is called once per handshake.
- Interpreted callbacks run when compiled `crypto/tls` calls them: `VerifyPeerCertificate` and `GetClientCertificate` closures, 16 concurrent handshakes against an mTLS fake, 0 failures, 16 calls each. A `VerifyPeerCertificate` that returns an error fails the dial with that error text.
- Empty `ServerName`: the dial address host is used for verification (`localhost:port` verifies a cert with SAN `localhost`; `127.0.0.1:port` against a cert with no IP SAN fails with `x509: cannot validate certificate for 127.0.0.1 because it doesn't contain any IP SANs`).
- `errors.As` reaches `*tls.CertificateVerificationError`, `x509.HostnameError`, `tls.RecordHeaderError`, and `*net.OpError`. A TLS alert sent by the peer (`remote error: tls: handshake failure`, `remote error: tls: certificate required`) is `*net.OpError` with `Op == "remote error"`; `errors.As` to `tls.AlertError` is **false** for it (Go wraps an unexported `alert`, see `ext_go_crypto-tls_client-dial`).
- `context.AfterFunc(ctx, conn.Close)` on a `*tls.Conn` whose read is blocked unblocks the read when ctx fires (cancel at 100ms, read returned at 100ms with `use of closed network connection`).
- Handshake cost, 50 sequential dials to a loopback fake (TLS 1.3, P-256): interpreted 0.75ms per TLS dial vs 0.24ms per plain TCP dial; compiled 0.72ms vs 0.26ms. The handshake runs in compiled `crypto/tls`, so interpretation adds almost nothing.
- TLS 1.2-only server (`MaxVersion` TLS 1.2): round trip succeeds and negotiates TLS 1.2.

Earlier probes for the same ticket (maintainer, 2026-11-21, same interpreter shape; `ticket` authority, recorded in the run's `requirement.md`): `DialContext` honours ctx and `NetDialer.Timeout` during the handshake (`context.DeadlineExceeded`); `SetDeadline` on `*tls.Conn` still yields `os.ErrDeadlineExceeded`; 32 goroutines x 100 commands and idle reuse after 500ms succeed; Traefik v3.7.11 loading a local plugin that dials TLS works against Redis, Redis ACL, Dragonfly, Dragonfly mTLS.

## Client-certificate rejection depends on the TLS version

Server requires a client cert, client sends none:

| Version | Where it surfaces | Error |
|---|---|---|
| TLS 1.2 | `DialContext` (handshake) | `remote error: tls: handshake failure` |
| TLS 1.3 | first read after the handshake | `remote error: tls: certificate required` |

Same on live Redis 7.4.10 (`tls-auth-clients` default) and Dragonfly v1.40.2 with `--tls_ca_cert_file`. On Windows loopback against the in-process fake, the TLS 1.3 first read sometimes returns `wsarecv: ... anulado` (connection reset, `Op == "read"`) instead of the alert: the reset can win over the alert record. Do not rely on seeing the alert on every platform.
