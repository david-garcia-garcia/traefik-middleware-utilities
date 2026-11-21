# Redis TLS

How Redis 7.4 (`redis:7-alpine`, 7.4.10 measured 2026-11-21) serves TLS and how a Go `crypto/tls` client sees its failure modes.

## Enabling TLS

`--tls-port <n>` with `--tls-cert-file`, `--tls-key-file`, `--tls-ca-cert-file`; `--port 0` disables the plaintext port. ([redis.io TLS](https://redis.io/docs/latest/operate/oss_and_stack/management/security/encryption/), `.sources/encryption.md`)

## Client certificates are required by default

"By default, Redis uses mutual TLS and requires clients to authenticate with a valid certificate"; `tls-auth-clients no` disables it (`optional` was used by the maintainer's earlier probe; the doc page does not list the values). (`.sources/encryption.md`)

A client without a certificate (live, `.sources/redis-tls-live-probe.md`):

- TLS 1.3: the handshake completes; the first read returns `remote error: tls: certificate required`.
- TLS 1.2 (client `MaxVersion`): `DialContext` fails with `remote error: tls: handshake failure`.

Go returns both as `*net.OpError{Op: "remote error"}` (`ext_go_crypto-tls_client-dial`).

## Mismatched transport

- TLS client to the plaintext port: Redis does not answer the ClientHello, so the dial ends on the dial timeout (`context deadline exceeded` after `DialTimeout`, 200ms here). It does **not** fail fast with `tls: first record does not look like a TLS handshake`; that fast failure is only seen against a peer that writes first.
- Plaintext client to the TLS port: the first read gets `EOF` in about 1ms.

Source: `.sources/redis-tls-live-probe.md` (authority source, live run).

## AUTH over TLS

AUTH replies are the same as on plaintext; ACL user forms are in `ext_redis_auth`.
