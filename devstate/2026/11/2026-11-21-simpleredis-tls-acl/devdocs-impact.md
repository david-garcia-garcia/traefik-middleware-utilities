# Devdocs impact
change: simpleredis-tls-acl

Pin: `origin/master...HEAD` at `fbfb20ef` (after `git-sync` merged `origin/master`). Packets read: `knowledge/devdocs/index.md`, `index_std_go.md`, `std_go_simpleredis.md`, `std_go_test-suites.md`.

## Units
- SimpleRedis TLS dial (`Config.TLS`) — subsystem — `simpleredis/pool.go` (`openNetConn`), `simpleredis/config.go`; spec `std_go_simpleredis_tls-handshake`
- SimpleRedis ACL-user AUTH (`Config.User`) — subsystem — `simpleredis/pool.go` (`authArgs`); spec `std_go_simpleredis_tcp-session`
- TLS rejection (`ErrTLS` / `redis:tls`) — subsystem — `simpleredis/resp.go` (`isTLSRejection`, `ioError`), `simpleredis/simpleredis.go`; spec `std_go_simpleredis_tcp-session`, `std_go_simpleredis_slog-events`
- Live TLS e2e engines (Go E2E Redis / Dragonfly TLS) — pattern — `.github/workflows/ci.yml`, `simpleredis/tls_e2e_test.go`, `simpleredis/yaegi_e2e_test.go`; spec `std_go_simpleredis_live-e2e`

## Findings
- [x] stale-usage  SimpleRedis TLS dial — `std_go_simpleredis`: retry Gotcha's do-not-retry list omits `redis:tls`; How-to live-proof list omits `SIMPLEREDIS_LIVE_*_TLS` / `SIMPLEREDIS_LIVE_TLS_DIR`
- [x] language-gap  TLS rejection — `std_go_simpleredis` has usage (How-to, Gotchas) but no Language term; "handshake failure" already means AUTH/SELECT after connect (`handshakeFailed`) in this packet
- [x] stale-usage  Live TLS e2e engines — `std_go_test-suites`: Gotcha on where handshake-failure proof lives names only SELECT 99 and `SIMPLEREDIS_LIVE_*_AUTH`, not the TLS engines' ACL WRONGPASS and `redis:tls` proofs

ACL-user AUTH: none. `std_go_simpleredis` How-to (`AUTH <User> <Pass>`, nopass form, `redis:noauth`, `User` not logged) and Gotcha (empty `User` and `Pass` skip AUTH) match `authArgs` and `TestAuthArgumentForms`.

## Produced
- `std_go_simpleredis` Language: added **TLS rejection** (definition from `isTLSRejection` and the `dial` comment; `_Avoid_`: TLS error, TLS timeout, handshake failure).
- `std_go_simpleredis` How-to: live-proof line names `SIMPLEREDIS_LIVE_REDIS_TLS` / `SIMPLEREDIS_LIVE_DRAGONFLY_TLS`, `SIMPLEREDIS_LIVE_TLS_DIR`, and what they prove (`tls_e2e_test.go`).
- `std_go_simpleredis` Gotchas: retry rule lists a TLS rejection (`redis:tls`) among the non-retried errors (`shouldRetry`, `TestTLSRejectionWithRetriesIsOneConnection`).
- `std_go_test-suites` Gotchas: handshake-failure proof line adds ACL-user WRONGPASS and `redis:tls` on `SIMPLEREDIS_LIVE_*_TLS`.

No index row changed (headings and descriptions still match). No issues or debt: every finding was produced.
