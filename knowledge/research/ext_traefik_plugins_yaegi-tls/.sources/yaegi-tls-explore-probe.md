---
url: "%TEMP%/claude/.../scratchpad/tlsexplore"
title: Throwaway Yaegi interp of a crypto/tls client (explore, 2026-11-21-simpleredis-tls-acl)
fetched: 2026-11-21
authority: source
---

Throwaway probe, not committed, not `go test` of the product. Host `go1.25.6 windows/amd64`, `github.com/traefik/yaegi v0.16.1`, `interp.New(Options{GoPath})`, `Use(stdlib.Symbols)` only. Interpreted package dials with `tls.Dialer{NetDialer: &net.Dialer{Timeout}, Config}.DialContext` and speaks RESP over `bufio`.

In-process compiled fakes (own CA, ECDSA P-256, PKCS#8):

- M1 Clone: `samePtr=false cloneServerName="localhost" clone=[ok +PONG TLS1.3] cloneCallbackCalls=1 mutatedOrig=[dial-err: tls: failed to verify certificate: x509: certificate signed by unknown authority] nilClone=true`
- M2 callbacks x16: `fails=0 verifyCalls=16 getClientCertCalls=16 rejectingCallback=[dial-err: pinned-mismatch]`
- M3 TLS1.2-only server: `ok +PONG TLS1.2`; TLS1.2 mTLS no cert: `dial-err: remote error: tls: handshake failure`
- M3c / M11b TLS1.3 mTLS no cert: first read `remote error: tls: certificate required` (`asOpError=true op="remote error" asAlertError=false`), or on some runs `wsarecv ... anulado` (`op="read"`)
- M4 ServerName empty: `localhost:port` ok; `127.0.0.1:port` with no IP SAN `asHostname=true asVerify=true`
- M5 AfterFunc close of blocked TLS read: `readReturnedAfter=100ms netErrClosed=true`
- M6 50 dials: interpreted TLS 752µs, TCP 237µs; compiled TLS 717µs, TCP 262µs
- M7f TLS client to plaintext fake: `elapsed=0ms asRecordHeaderError=true err=tls: first record does not look like a TLS handshake`
- M11 TLS1.2 alert at dial: `asAlertError=false asOpError=true op="remote error"`

Live containers (`redis:7-alpine` 7.4.10, Dragonfly v1.40.2): see `ext_redis_tls/.sources/redis-tls-live-probe.md` and `ext_dragonfly_tls/.sources/dragonfly-tls-live-probe.md`.
