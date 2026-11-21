## Why

SimpleRedis only dials plain TCP and only sends one-argument `AUTH <pass>`, so a middleware cannot reach a Redis or Dragonfly that sits outside a private network (TLS) or that uses ACL users. The live contract even forbids TLS (`std_go_simpleredis_tcp-session`, "MUST NOT use TLS"). A library user asked for both; the maintainer confirmed that scope and left pipelining and multi-endpoint HA out.

## What Changes

- `Config.TLS *tls.Config`. nil keeps today's plain TCP. Non-nil: `New` stores `cfg.TLS.Clone()`, and `dial` runs `tls.Dialer{NetDialer: &net.Dialer{Timeout: DialTimeout}, Config}.DialContext`. CA, system roots, `ServerName`, client certificates (mTLS), versions and callbacks are the caller's `tls.Config`; the library adds no parallel fields. An empty `ServerName` is inferred by `crypto/tls` from the host part of `Host`.
- `DialTimeout` now bounds one dial attempt as a whole: TCP connect plus the TLS handshake. No new timeout knob.
- `Config.User`. With `User`, each new dial sends `AUTH <user> <pass>` (also `AUTH <user> ""` when `Pass` is empty). Without `User`, behaviour is unchanged (`AUTH <pass>` when `Pass` is set).
- New exported sentinel `ErrTLS` with text `RedisTLS = "redis:tls"`. It covers a rejected server certificate, a TLS alert from the server (during the handshake, or on the first read, which is where TLS 1.3 reports a refused client certificate) and a peer that answers the ClientHello with non-TLS bytes. It does not wrap `ErrUnreachable`, is not retried, and the socket is closed. A slow or silent peer (handshake past `DialTimeout`) stays `redis:unreachable` and retryable; `CommandTimeout` expiry during the handshake is `redis:timeout`, as on plain TCP.
- Logging: a TLS failure is logged at Error by the site that saw it (`dial: redis:tls`, `do: redis:tls`), with the `crypto/tls` error text on a `tls_error` attribute. `Pass`, key names and values stay off log lines; `User` is not added to any line.
- Tests: in-process TLS fake (own CA, mTLS, TLS 1.2 only, failure classes), ACL-user AUTH forms on the fake, Yaegi TLS, and live Go E2E against TLS + ACL-user Redis and Dragonfly in the existing `Go E2E Redis` / `Go E2E Dragonfly` CI jobs. Certificate material is generated at test-setup time with stdlib and is never committed.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md`: TLS leaves `_Avoid_`; usage for `TLS`, `User`, `ErrTLS`.

No **BREAKING** change: both new `Config` fields are additive and their zero values keep current behaviour.

## Capabilities

### New Capabilities

- `std_go_simpleredis_tls-handshake`: how a TLS dial fails — which failures become `redis:tls`, that `redis:tls` is not retried and is not `redis:unreachable`, which failures stay `redis:unreachable`, Yaegi-safe classification, and the interpreted TLS proof.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: `First command dials TCP` (TLS over that TCP connection when `Config.TLS` is set; Unix sockets still forbidden), `AUTH and SELECT run once per dial` (ACL-user two-argument form), `New records settings and does not dial` (copies `User`, clones `TLS`, `DialTimeout` = TCP connect plus handshake, `crypto/tls` allowed).
- `std_go_simpleredis_slog-events`: `redis:tls` joins the package-owned sentinels in `A peer reply is not published`; a TLS failure is Error at the site that saw it.
- `std_go_simpleredis_live-e2e`: live TLS + ACL-user proof on Redis and Dragonfly, compiled and Yaegi.

## Impact

- Code: `simpleredis/config.go`, `simpleredis/simpleredis.go` (sentinel, `New`), `simpleredis/pool.go` (`dial`), `simpleredis/resp.go` (`ioError`, `do` log lines), new and extended `simpleredis/*_test.go`.
- CI: `.github/workflows/ci.yml` jobs `e2e-redis` and `e2e-dragonfly` (generate certificates, start one TLS + ACL container each, export the TLS live env vars).
- Main specs after archive: `std_go_simpleredis_tcp-session`, `std_go_simpleredis_slog-events`, `std_go_simpleredis_live-e2e`, new `std_go_simpleredis_tls-handshake`.
- Usage: `knowledge/devdocs/std_go_simpleredis.md`, `knowledge/devdocs/std_go_test-suites.md` (new live env vars).
- Consumers: `tokenbucket` and `windowcounter` take a built `*SimpleRedis`; no change. The 197 `Config` literals keep compiling.
- Out of scope: pipelining; multiple endpoints / Sentinel / Cluster; library-level helpers or validation for `tls.Config` callbacks (`VerifyPeerCertificate`, `GetClientCertificate` work because the type carries them, but are not promised); TLS or ACL settings on `tokenbucket` / `windowcounter`; Unix sockets; changing TLS or credentials after `New`; a Traefik/Pester TLS route (`docker-compose.yml`, `e2e/simpleredisprobe`, `Test-Integration.ps1` stay as they are).
