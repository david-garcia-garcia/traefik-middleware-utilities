---
url: "%TEMP%/claude/.../scratchpad/tlsexplore (live mode)"
title: Live Dragonfly v1.40.2 TLS probe (explore, 2026-11-21-simpleredis-tls-acl)
fetched: 2026-11-21
authority: source
---

Image `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`, containers removed after the run. Certs PKCS#8 ECDSA P-256 via `docker cp`.

- `dragonfly --tls --tls_cert_file --tls_key_file` (no CA, no requirepass): exits at start with `E... server_family.cc:1011] TLS configured but no authentication method is used!`
- Same plus `--requirepass=topsecret`: starts. With `--tls_ca_cert_file` and no requirepass: starts.
- `redis-cli --tls ... ACL SETUSER app on >secret ~* +@all` → OK on both.
- CA set, client without cert, TLS 1.3: first read `remote error: tls: certificate required`; client MaxVersion TLS 1.2: dial `remote error: tls: handshake failure`.
- No CA (requirepass): `AUTH app secret` +OK, `AUTH app wrong` and `AUTH app ""` → `WRONGPASS invalid username-password pair or user is disabled.`
- CA with client cert: `AUTH app secret` +OK.
- Plain client PING to the TLS port: `-ERR Bad TLS header, double check if you enabled TLS for your client.` in ~1ms.
