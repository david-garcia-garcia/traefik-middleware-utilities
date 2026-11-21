---
url: "%TEMP%/claude/.../scratchpad/tlsexplore (live mode)"
title: Live redis:7-alpine 7.4.10 TLS probe (explore, 2026-11-21-simpleredis-tls-acl)
fetched: 2026-11-21
authority: source
---

Containers (removed after the run): plaintext `redis:7-alpine`; TLS `redis-server --port 0 --tls-port 6380 --tls-cert-file --tls-key-file --tls-ca-cert-file --requirepass topsecret --user app on >secret ~* &* +@all --user free on nopass ~* +@all` (certs PKCS#8 ECDSA P-256, `docker cp`). `redis-server --version` 7.4.10. Client: Yaegi v0.16.1 interpreted `tls.Dialer.DialContext`, `DialTimeout` 200ms, ctx 900ms.

- TLS client to plaintext port: `elapsed=200ms err=context deadline exceeded` (`asRecordHeaderError=false`)
- Plain client PING to TLS port: `elapsed=1ms err=EOF`
- No client cert, TLS 1.3: first read `remote error: tls: certificate required`
- No client cert, client MaxVersion TLS 1.2: dial `remote error: tls: handshake failure`
- mTLS, no AUTH: `NOAUTH Authentication required.` (TLS 1.3 and forced TLS 1.2)
- ServerName empty, addr `127.0.0.1:port`, cert IP SAN 127.0.0.1: ok
- `AUTH app secret` +OK; `AUTH app wrong` / `AUTH ghost secret` / `AUTH app ""` → `WRONGPASS invalid username-password pair or user is disabled.`; `AUTH free ""` (nopass) +OK; `AUTH default topsecret` +OK
