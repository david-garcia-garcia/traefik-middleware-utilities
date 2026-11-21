# Dragonfly TLS

How Dragonfly v1.40.2 (dest e2e image) serves TLS, measured 2026-11-21.

## Flags

`--tls`, `--tls_cert_file`, `--tls_key_file`, optional `--tls_ca_cert_file`; `--no_tls_on_admin_port` relaxes the admin port only. ([Dragonfly flags](https://www.dragonflydb.io/docs/managing-dragonfly/flags), `.sources/flags.md`)

## TLS needs an authentication method or the server will not start

`--tls` with cert and key but neither `--requirepass` nor `--tls_ca_cert_file` exits at startup: `TLS configured but no authentication method is used!`. Adding either one starts the server. The docs page does not state this. (`.sources/dragonfly-tls-live-probe.md`, authority source, live run)

## A CA file makes client certificates mandatory

With `--tls_ca_cert_file`, a client without a certificate fails like Redis: TLS 1.3 first read `remote error: tls: certificate required`; TLS 1.2 dial `remote error: tls: handshake failure`. The flag doc only says the CA is used "to validate tls connections". (`.sources/flags.md`, `.sources/dragonfly-tls-live-probe.md`)

## Key format

Dragonfly did not load an EC key in SEC1 (`EC PRIVATE KEY`); PKCS#8 (`PRIVATE KEY`) loads. Maintainer probe 2026-11-21 (`ticket` authority, run `requirement.md`); PKCS#8 confirmed loading in this run.

## Mismatched transport

A plaintext client on the TLS port gets a RESP error reply `-ERR Bad TLS header, double check if you enabled TLS for your client.` (not a closed socket, unlike Redis). (`.sources/dragonfly-tls-live-probe.md`)

## ACL users over TLS

`ACL SETUSER app on >secret ~* +@all` then `AUTH app secret` works over TLS; see `ext_dragonfly_auth`.
