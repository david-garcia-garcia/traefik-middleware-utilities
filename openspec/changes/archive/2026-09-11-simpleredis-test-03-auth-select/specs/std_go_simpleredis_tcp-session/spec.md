## ADDED Requirements

### Requirement: Handshake AUTH or SELECT failure closes and is not pooled
When AUTH on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool. AUTH-class prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) SHALL map to `redis:noauth`. When SELECT on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool, and SHALL return that error text. `ERR DB index is out of range` MUST NOT map to `redis:noauth`. AUTH SHALL run before SELECT when both password and database are non-empty. A handshake failure SHALL surface one error to the caller and MUST NOT open a second TCP connection for that command. In-process handshake-failure tests SHALL use a fake whose AUTH and SELECT replies are configurable (default success so existing success tests stay). Live Redis and Dragonfly tests SHALL prove the cases each dest engine supports and SHALL skip when those engines are unset.

#### Scenario: Fake AUTH rejected maps to redis:noauth and is not pooled
- **WHEN** the client is Inited with a non-empty password and an empty database
- **AND** the fake replies to AUTH with an AUTH-class prefix (`NOAUTH`, `WRONGPASS`, `NOPERM`, or `ERR Client sent AUTH`)
- **AND** a command is issued
- **THEN** the command returns `redis:noauth`
- **AND** the idle pool is empty
- **AND** the fake observes that the client closed the socket
- **AND** the fake accepted one TCP connection

#### Scenario: Fake SELECT rejected after AUTH is not pooled
- **WHEN** the client is Inited with a password and database `99`
- **AND** the fake replies `+OK` to AUTH and `-ERR DB index is out of range` to SELECT
- **AND** a command is issued
- **THEN** AUTH is sent before SELECT
- **AND** the command returns `ERR DB index is out of range`
- **AND** the idle pool is empty
- **AND** the fake observes that the client closed the socket
- **AND** the fake accepted one TCP connection

#### Scenario: Live SELECT 99 on Redis and Dragonfly
- **WHEN** dest Redis and Dragonfly are reachable without a password
- **AND** the client is Inited with database `99`
- **AND** a command is issued
- **THEN** each engine returns `ERR DB index is out of range`
- **AND** the idle pool is empty
- **WHEN** those engines are unset
- **THEN** the live SELECT tests skip

#### Scenario: Live wrong password on Redis and Dragonfly with requirepass
- **WHEN** dest Redis and Dragonfly are reachable with requirepass set
- **AND** the client is Inited with a wrong password
- **AND** a command is issued
- **THEN** each engine returns `redis:noauth`
- **AND** the idle pool is empty
- **WHEN** those passworded engines are unset
- **THEN** the live wrong-password tests skip
