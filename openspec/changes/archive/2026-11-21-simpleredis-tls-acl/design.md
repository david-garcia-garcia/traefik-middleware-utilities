## Context

See proposal.md for why. Today `dial` (`simpleredis/pool.go`) is `net.Dialer{Timeout: DialTimeout}.DialContext(ctx, "tcp", host)`, then `AUTH <pass>` and `SELECT`, and every dial error is `redis:unreachable` (retried by `shouldRetry` in `commands_exec.go` through identity with `errUnreachable`). `ioError` (`simpleredis/resp.go`, single caller `ioOrContext`) maps `os.ErrDeadlineExceeded` to `redis:timeout` and every other I/O error to `redis:unreachable`. `New` copies `Config` field by field. Log lines follow `std_go_simpleredis_slog-events` (`<site>: <cause>`, never peer text).

Measurements and proceed policies live in the run's `explore.md` (M1–M11) and in `knowledge/research/ext_traefik_plugins_yaegi-tls/`, `ext_go_crypto-tls_client-dial/`, `ext_redis_tls/`, `ext_dragonfly_tls/`, `ext_redis_auth/`, `ext_dragonfly_auth/`. Constraint: library code stays stdlib-only and interpretable by Yaegi v0.16.1 with `stdlib.Symbols`; CI builds with Go 1.21.

## Goals / Non-Goals

**Goals:**
- One TLS surface owned by `crypto/tls` (`*tls.Config`), frozen at `New`.
- One dial path with a TLS branch; `DialTimeout` keeps being the only dial bound.
- A deterministic TLS failure is one non-retried sentinel, seen the same way whether it surfaces in the handshake or on the first read.
- ACL-user AUTH without changing the password-only form.

**Non-Goals:**
- Wrapping or validating `tls.Config` (no `CAFile`/`CertFile` fields, no callback policy).
- Distinguishing a TLS client that talks to a silent plaintext Redis from a slow peer.
- TLS session resumption, handshake-cost tuning, a handshake timeout knob.
- Traefik/Pester TLS routes.

## Decisions

1. **`Config.TLS *tls.Config`, cloned in `New`.** nil = plain TCP. `New` stores `cfg.TLS.Clone()` (nil-safe; measured under Yaegi, M1). Same shape as go-redis `Options.TLSConfig`. Alternative: narrow fields (`CAFile`, `CertPEM`, `ServerName`, `Insecure`) — rejected: a parallel abstraction over `tls.Config` that cannot express system roots, versions, or ciphers without growing. Alternative: deep copy — rejected: `Clone` is the documented owner; shallowness is documented on the field.

2. **Dial with `tls.Dialer{NetDialer: &net.Dialer{Timeout: DialTimeout}, Config: sr.tlsConfig}.DialContext(ctx, "tcp", host)`.** It honours the command ctx and wraps connect + handshake in the one `NetDialer.Timeout` (research `ext_go_crypto-tls_client-dial`). `tls.DialWithDialer` is rejected (ignores ctx). `ServerName` is not rebuilt from `Host`: `crypto/tls` infers it from the address when empty (M4), so the identity owner stays `crypto/tls`. Both branches keep the same following steps (`pooledConn`, AUTH, SELECT) so TLS and plain differ only in how the `net.Conn` is obtained.

3. **`Config.User`; AUTH when `User != "" || Pass != ""`.** With `User`: `AUTH user pass` (also `AUTH user ""`). Without: `AUTH pass` as today. Alternative: ignore `User` without `Pass` like go-redis v9.22.0 — rejected: the session would silently run as `default` (`ext_go-redis_auth_username`). Alternative: `New` error for `User` without `Pass` — rejected: a `nopass` ACL user is valid (measured `+OK`, M9).

4. **Sentinel `ErrTLS` / `RedisTLS = "redis:tls"`, `errors.New`, does not wrap `ErrUnreachable`.** Returned bare (like `ErrNoAuth`), so `Error()` is exactly `redis:tls` and `shouldRetry` falls through: not `errUnreachable` by identity, not a retryable Redis reply prefix, so no change to `shouldRetry` is needed. The `crypto/tls` text goes on the log line (`tls_error`), not into the returned error, keeping every exported sentinel text exact (`std_go_simpleredis_slog-events`). No `IsTLS` helper: `ErrNoAuth` has none either; callers use `errors.Is`.

5. **One classifier, two call sites.** A small function (for example `isTLSRejection(err error) bool`) answers "is this a deterministic TLS rejection" with `errors.As` to `*tls.CertificateVerificationError`, `tls.RecordHeaderError`, and `*net.OpError` with `Op == "remote error"`. `dial` calls it on a `DialContext` error (after the existing `contextStop` check, so caller cancel/deadline still wins); `ioError` calls it before falling back to `redis:unreachable`, which covers the TLS 1.3 client-certificate alert on the first read in both the AUTH read and the first-command read. Plain TCP never produces those types, so the plain path is unchanged. Rejected: `errors.As(tls.AlertError)` — false for a server alert under Yaegi (M11). Rejected: string matching on `remote error`. Rejected: a handshake-only sentinel — misses the TLS 1.3 first-read case.

6. **Logging.** `dial` handshake rejection: `sr.logger.Error("dial: "+RedisTLS, "tls_error", err.Error())`. `do` read/write branch: when the mapped error is `errTLS`, `Error("do: "+RedisTLS, "verb", …, "tls_error", err.Error())` instead of the current `redis:unreachable` line. `dial` after a failed AUTH/SELECT whose error is `ErrTLS`: `Warn("dial: "+RedisTLS, "verb", …)`, the same shape the `redis:noauth` branch already has. No new `simpleredis_*` event, no helper, no exported strings.

7. **Test fixtures: one certificate generator, in test code.** `simpleredis/tls_fixture_test.go` builds a P-256 CA, a server certificate (SAN `localhost`, `redis-tls`, `dragonfly-tls`, IP `127.0.0.1` — and a no-IP-SAN variant for the name test) and a client certificate, keys in PKCS#8 (Dragonfly rejects SEC1). The in-process TLS fake wraps the existing `startFakeRedis` listener with `tls.NewListener` so RESP behaviour and counters stay one implementation. A test `TestWriteTLSFixtureFiles` writes `ca.crt`, `server.crt`, `server.key`, `client.crt`, `client.key` to `SIMPLEREDIS_TLS_FIXTURE_OUT` and skips when that variable is unset; CI runs it before starting the TLS containers. Alternative: a nested `e2e/tlscerts` module — rejected: a second generator for the same job. Alternative: committed PEM files — rejected (private keys and expiry in the repo).

8. **Live composition (CI only, `.github/workflows/ci.yml`).** `e2e-redis`: generate the fixture into `$RUNNER_TEMP/simpleredis-tls`, `docker run` `redis:7-alpine` on `6383` with `--port 0 --tls-port 6379 --tls-cert-file --tls-key-file --tls-ca-cert-file` (client certificates required, Redis default), then create ACL user `app` (`redis-cli --tls … ACL SETUSER app on >apppass ~* &* +@all`), and export `SIMPLEREDIS_LIVE_REDIS_TLS=127.0.0.1:6383`, `SIMPLEREDIS_LIVE_TLS_DIR`. `e2e-dragonfly`: same with Dragonfly v1.40.2 on `6384`, `--tls --tls_cert_file --tls_key_file --tls_ca_cert_file` (client certificates required; also satisfies Dragonfly's "TLS needs an authentication method" start check) plus `ACL SETUSER app …`, exporting `SIMPLEREDIS_LIVE_DRAGONFLY_TLS=127.0.0.1:6384`. The ACL user password is a fixed test constant shared by the CI step and the test file, like `secret` for the AUTH pair. Key files are written world-readable so the container user can load them. Local developers run the same `docker run` lines by hand; `docker-compose.yml` is untouched.

## Risks / Trade-offs

- [TLS client against a silent plaintext Redis waits `DialTimeout` per attempt and looks like a slow peer] → Accepted (M7); documented in the usage packet. It stays `redis:unreachable` and inside `CommandTimeout`.
- [On Windows loopback a TCP reset can beat the TLS 1.3 alert (M11), so the first-read case may surface as `redis:unreachable` and retry once] → Production servers are not affected; the in-process fake must make the alert observable (for example: keep the server side open and drain the client's bytes until the client closes, instead of closing right after the alert). If a test still cannot see the alert reliably on Windows, assert the TLS 1.3 first-read scenarios on the CI platform and document the platform limit instead of loosening the classifier.
- [Shallow clone: the caller can still mutate `RootCAs` pool contents or `Certificates` elements] → Documented on `Config.TLS`; `Clone` is the stdlib owner and is concurrency-safe.
- [`tls.Conn.Close` sends `close_notify` with a 5 s write deadline] → Only blocks when the kernel send buffer is full (inference on Go source, explore row). Accepted; `pooledConn.close` unchanged.
- [Callbacks on `tls.Config` run interpreted code inside compiled `crypto/tls`] → Measured working (M2); carried by the type, not promised by a spec.
- [A `VerifyPeerCertificate` error or a client-side protocol error is not one of the classified types] → It stays `redis:unreachable` and retries once; acceptable because those paths are caller-owned or rare, and adding text matching would break the "no text parsing" rule.

## Migration Plan

Additive fields; zero values keep plain TCP and password-only AUTH. Rollback is a revert. No persistent storage involved.

## Open Questions

None. Proceed policies are rows on the run's `explore.md`.
