---
url: https://github.com/redis/go-redis/blob/v9.22.0/options.go
title: go-redis v9.22.0 Options.TLSConfig and NewDialer
fetched: 2026-11-21
authority: source
ref: github.com/redis/go-redis@v9.22.0:options.go
---

- Field: `// TLSConfig to use. When set, TLS will be negotiated.` `TLSConfig *tls.Config`
- `NewDialer`: `netDialer := &net.Dialer{Timeout: opt.DialTimeout, KeepAliveConfig: ...}`; `if opt.TLSConfig == nil { return netDialer.DialContext(ctx, network, addr) }`; `return tls.DialWithDialer(netDialer, network, addr, opt.TLSConfig)`.
- `ParseURL` with scheme `rediss`: `o.TLSConfig = &tls.Config{ServerName: h, MinVersion: tls.VersionTLS12}`.
- No `Clone()` of `TLSConfig` in `NewDialer`.
