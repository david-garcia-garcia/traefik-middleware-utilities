## ADDED Requirements

### Requirement: Live TLS and ACL user on Redis and Dragonfly
Compiled tests SHALL run against each TLS address that is set (`SIMPLEREDIS_LIVE_REDIS_TLS`, `SIMPLEREDIS_LIVE_DRAGONFLY_TLS`), loading the CA and client certificate from `SIMPLEREDIS_LIVE_TLS_DIR`. Each engine SHALL require a client certificate and have ACL user `app`. Skip rules SHALL match the AUTH pair: skip under `-short` or when both addresses are unset; one set address runs that engine only.

#### Scenario: Live mutual TLS with an ACL user
- **WHEN** a TLS live address is set and tests are not `-short`
- **AND** the client trusts the test CA, presents the test client certificate, and sets `User` `app` with that user's password
- **THEN** `Set` then `Get` round-trip on that engine

#### Scenario: Live wrong ACL password over TLS
- **WHEN** a TLS live address is set and tests are not `-short`
- **AND** the client sets `User` `app` with a wrong password
- **THEN** the command returns `redis:noauth`
- **AND** the socket is not pooled

#### Scenario: Live missing client certificate
- **WHEN** a TLS live address is set and tests are not `-short`
- **AND** the client trusts the test CA but presents no certificate
- **THEN** the command returns `redis:tls`

#### Scenario: Live untrusted server certificate
- **WHEN** a TLS live address is set and tests are not `-short`
- **AND** the client's `RootCAs` does not contain the test CA
- **THEN** the command returns `redis:tls`

#### Scenario: TLS addresses unset
- **WHEN** both TLS live addresses are unset
- **THEN** the live TLS tests skip

### Requirement: Yaegi live TLS and ACL user on both engines
Tests that import Yaegi v0.16.1 SHALL prove interpreted `New` with `Config.TLS` (test CA and client certificate) and `User`, plus `Set` and `Get`, against each live TLS engine whose address is set. They MUST use GOPATH with stdlib symbols only and `useunsafe` false, MUST NOT start Traefik, and SHALL follow the compiled TLS live skip rules.

#### Scenario: Interpreted live TLS
- **WHEN** a TLS live address is set and tests are not `-short`
- **THEN** the Yaegi live TLS probe round-trips `Set` then `Get` on that engine
