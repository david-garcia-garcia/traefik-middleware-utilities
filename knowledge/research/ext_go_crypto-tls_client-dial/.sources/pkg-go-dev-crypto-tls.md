---
url: https://pkg.go.dev/crypto/tls
title: tls package - crypto/tls - Go Packages
fetched: 2026-11-21
authority: official
---

- `Config.Clone`: "Clone returns a shallow clone of c or nil if c is nil." "It is safe to clone a Config that is being used concurrently by a TLS client or server."
- `Config.ServerName`: "ServerName is used to verify the hostname on the returned certificates unless InsecureSkipVerify is given."
- `Config.MinVersion`: "By default, TLS 1.2 is currently used as the minimum."
- `DialWithDialer`: "Any timeout or deadline given in the dialer apply to connection and TLS handshake as a whole." "DialWithDialer uses context.Background internally; to specify the context, use Dialer.DialContext with NetDialer set to the desired dialer."
- `CertificateVerificationError`: "returned when certificate verification fails during the handshake."
- `Config.VerifyPeerCertificate`: "if not nil, is called after normal certificate verification by either a TLS client or server."
- `Config.GetClientCertificate`: "if not nil, is called when a server requests a certificate from a client."
