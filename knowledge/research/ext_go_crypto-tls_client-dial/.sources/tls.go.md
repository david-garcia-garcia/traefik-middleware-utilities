---
url: https://github.com/golang/go/blob/go1.25.6/src/crypto/tls/tls.go
title: crypto/tls dial and Dialer.DialContext (Go 1.25.6)
fetched: 2026-11-21
authority: source
ref: golang/go@go1.25.6:src/crypto/tls/tls.go
---

`func dial(ctx, netDialer, network, addr, config)`:

- `if netDialer.Timeout != 0 { ctx, cancel = context.WithTimeout(ctx, netDialer.Timeout) }` — the timeout covers TCP connect and `HandshakeContext`.
- `hostname := addr[:colonPos]`; `// If no ServerName is set, infer the ServerName from the hostname we're connecting to.` → `c := config.Clone(); c.ServerName = hostname`.
- `conn := Client(rawConn, config); if err := conn.HandshakeContext(ctx); err != nil { rawConn.Close(); return nil, err }`.

`(*Dialer).DialContext` calls `dial(ctx, d.netDialer(), network, addr, d.Config)`; doc: "Once successfully connected, any expiration of the context will not affect the connection."
