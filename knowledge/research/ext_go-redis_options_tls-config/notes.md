# go-redis Options.TLSConfig

How go-redis v9 exposes and dials TLS, the shape SimpleRedis copies without importing the library.

- `Options.TLSConfig *tls.Config`; nil means plain TCP, non-nil means TLS. The caller's `*tls.Config` is used as is: no `Clone`, no rejected fields, so callbacks on it (`VerifyPeerCertificate`, `GetClientCertificate`) are honoured.
- The dial is `tls.DialWithDialer(&net.Dialer{Timeout: DialTimeout}, network, addr, TLSConfig)`. `DialWithDialer` uses `context.Background` internally, so go-redis's TLS handshake honours `DialTimeout` but not the command ctx (`ext_go_crypto-tls_client-dial`).
- `rediss://` URLs build `&tls.Config{ServerName: <host>, MinVersion: tls.VersionTLS12}`.

Source: `github.com/redis/go-redis@v9.22.0:options.go`, `.sources/options.go.md` (read from the local module cache, not cloned).
