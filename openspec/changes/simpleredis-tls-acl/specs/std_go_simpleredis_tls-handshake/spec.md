## Purpose

Defines how a SimpleRedis TLS dial fails: which TLS failures are the non-retried `redis:tls` sentinel, which stay `redis:unreachable`, how they are told apart safely under Yaegi, and the interpreted proof that a TLS session works.

## ADDED Requirements

### Requirement: A TLS rejection returns redis:tls
With a TLS configuration set, the session SHALL return `ErrTLS` (`Error()` text `redis:tls`, exported constant `RedisTLS`) when the client rejects the server certificate, when the server sends a TLS alert during the handshake or on a later read, or when the peer answers the ClientHello with bytes that are not a TLS record. `errors.Is(err, ErrTLS)` SHALL match for compiled and Yaegi-interpreted callers.

#### Scenario: Unknown CA
- **WHEN** `Config.TLS.RootCAs` does not trust the TLS fake's CA
- **AND** a command is issued
- **THEN** the command returns `redis:tls`
- **AND** `errors.Is` matches `ErrTLS`

#### Scenario: Server name mismatch
- **WHEN** `Config.TLS` trusts the fake's CA and sets `ServerName` to a name the fake's certificate does not carry
- **THEN** the command returns `redis:tls`
- **WHEN** `ServerName` is empty and `Host` is `127.0.0.1:<port>` while the certificate has no IP SAN
- **THEN** the command returns `redis:tls`

#### Scenario: Client certificate refused in the TLS 1.2 handshake
- **WHEN** the TLS fake requires a client certificate and allows at most TLS 1.2
- **AND** `Config.TLS` has no certificate
- **THEN** the command returns `redis:tls`

#### Scenario: Client certificate refused on the first TLS 1.3 read without a password
- **WHEN** the TLS fake requires a client certificate and negotiates TLS 1.3
- **AND** `Config.TLS` has no certificate and `Pass` and `User` are empty
- **THEN** the first command returns `redis:tls`, not `redis:unreachable`

#### Scenario: Client certificate refused on the first TLS 1.3 read during AUTH
- **WHEN** the TLS fake requires a client certificate and negotiates TLS 1.3
- **AND** `Config.TLS` has no certificate and `Pass` is set
- **THEN** the command returns `redis:tls`

#### Scenario: Peer answers the ClientHello with plain text
- **WHEN** `Config.TLS` is set and the peer is a plain TCP server that replies to the first bytes it reads with a RESP error line
- **THEN** the command returns `redis:tls`

### Requirement: redis:tls is not retried and is not unreachable
`ErrTLS` MUST NOT wrap `ErrUnreachable`; `IsUnreachable` SHALL be false for it. A `redis:tls` failure MUST NOT be retried and MUST NOT open a second TCP connection for that command, whatever `MaxRetries` is. The socket SHALL be closed and MUST NOT be appended to the idle pool. The returned error SHALL be the sentinel itself, so its `Error()` text is exactly `redis:tls`.

#### Scenario: Rejection with retries on is one connection
- **WHEN** the client is created with `MaxRetries: 3` and MinRetryBackoff off
- **AND** the TLS fake's certificate is not trusted
- **AND** a command is issued
- **THEN** the command returns an error whose `Error()` text is exactly `redis:tls`
- **AND** `IsUnreachable` is false
- **AND** the fake accepted one TCP connection
- **AND** the idle pool is empty

#### Scenario: First-read alert is not retried
- **WHEN** the TLS fake refuses a missing client certificate on the first TLS 1.3 read
- **AND** the client is created with `MaxRetries: 1` and no password
- **THEN** the command returns `redis:tls`
- **AND** the fake accepted one TCP connection

### Requirement: A slow or silent TLS peer stays redis:unreachable
A TCP connect failure, a `DialTimeout` or command-context expiry during the TLS handshake, and any dial error not named in `A TLS rejection returns redis:tls` SHALL return `redis:unreachable` and MAY be retried exactly as a plain TCP dial failure is. A caller cancel or deadline during the handshake SHALL surface as that context error, as on plain TCP.

#### Scenario: TLS client against a silent plain peer
- **WHEN** `Config.TLS` is set and the peer accepts TCP and never writes
- **AND** the client is created with `MaxRetries: 1`, MinRetryBackoff off, and `DialTimeout` 100 milliseconds
- **THEN** the command returns `redis:unreachable`
- **AND** `IsUnreachable` is true
- **AND** the peer accepted two TCP connections

#### Scenario: TLS host refuses TCP
- **WHEN** `Config.TLS` is set and `Host` is `127.0.0.1:1`
- **THEN** the command returns `redis:unreachable`

### Requirement: TLS failures are classified without net.Error or AlertError
The session SHALL tell TLS failures apart with `errors.As` against concrete types: `*tls.CertificateVerificationError`, `tls.RecordHeaderError`, and `*net.OpError` whose `Op` is `remote error`. It MUST NOT type-assert the `net.Error` interface and MUST NOT depend on `tls.AlertError`, which does not match a server alert under Yaegi v0.16.1. Classification MUST NOT parse error text.

#### Scenario: Interpreted caller sees the same class
- **WHEN** interpreted code under Yaegi v0.16.1 (stdlib symbols only) creates a client whose `Config.TLS` does not trust the TLS fake's CA
- **AND** issues a command
- **THEN** `errors.Is(err, simpleredis.ErrTLS)` is true in the interpreted code
- **AND** `simpleredis.IsUnreachable(err)` is false

### Requirement: A TLS session works under Yaegi
Interpreted code under Yaegi v0.16.1 with stdlib symbols only and `useunsafe` false SHALL build a client with `Config.TLS` (own CA and a client certificate) and `Config.User`, and SHALL complete commands over TLS against the in-process TLS fake. Those tests MUST NOT start Traefik.

#### Scenario: Interpreted mutual TLS with an ACL user
- **WHEN** interpreted code calls `New` with a `Config.TLS` that trusts the fake's CA and carries a client certificate, `User` `app`, and `Pass` `secret`
- **AND** calls `Set` then `Get`
- **THEN** `Get` returns the stored value
- **AND** the fake received `AUTH app secret` over TLS
