## MODIFIED Requirements

### Requirement: Handshake AUTH or SELECT failure closes and is not pooled
When AUTH on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool. AUTH-class prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) SHALL map to `redis:noauth`. When SELECT on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool, and SHALL return that error text. `ERR DB index is out of range` MUST NOT map to `redis:noauth`. AUTH SHALL run before SELECT when both password and database are non-empty. A handshake failure SHALL surface one error to the caller and MUST NOT open a second TCP connection for that command. That MUST NOT-redial rule includes AUTH or SELECT peer close with no reply, AUTH `-LOADING Redis is loading the dataset in memory`, and AUTH `-ERR max number of clients reached`. TCP dial refuse before AUTH/SELECT MAY still retry. A command that receives `LOADING ` after a successful handshake MAY still retry. In-process handshake-failure tests SHALL use a fake whose AUTH and SELECT replies are configurable (default success so existing success tests stay). Live Redis and Dragonfly tests SHALL prove the cases each dest engine supports and SHALL skip when those engines are unset.

#### Scenario: Fake AUTH rejected maps to redis:noauth and is not pooled
- **WHEN** the client is created with `New` with a non-empty password and an empty database
- **AND** the fake replies to AUTH with an AUTH-class prefix (`NOAUTH`, `WRONGPASS`, `NOPERM`, or `ERR Client sent AUTH`)
- **AND** a command is issued
- **THEN** the command returns `redis:noauth`
- **AND** the idle pool is empty
- **AND** the fake observes that the client closed the socket
- **AND** the fake accepted one TCP connection

#### Scenario: Fake SELECT rejected after AUTH is not pooled
- **WHEN** the client is created with `New` with a password and database `99`
- **AND** the fake replies `+OK` to AUTH and `-ERR DB index is out of range` to SELECT
- **AND** a command is issued
- **THEN** AUTH is sent before SELECT
- **AND** the command returns `ERR DB index is out of range`
- **AND** the idle pool is empty
- **AND** the fake observes that the client closed the socket
- **AND** the fake accepted one TCP connection

#### Scenario: Fake AUTH close without reply is not redialed
- **WHEN** the client is created with `New` with a non-empty password, `MaxRetries: 1`, and MinRetryBackoff off
- **AND** the peer accepts TCP, reads AUTH, and closes with no reply
- **AND** a command is issued
- **THEN** the command returns an error
- **AND** the peer accepted one TCP connection

#### Scenario: Fake SELECT close without reply after AUTH is not redialed
- **WHEN** the client is created with `New` with a password, a database, `MaxRetries: 1`, and MinRetryBackoff off
- **AND** the peer accepts TCP, replies `+OK` to AUTH, reads SELECT, and closes with no reply
- **AND** a command is issued
- **THEN** the command returns an error
- **AND** the peer accepted one TCP connection

#### Scenario: Fake AUTH LOADING is not redialed
- **WHEN** the client is created with `New` with a non-empty password, `MaxRetries: 1`, and MinRetryBackoff off
- **AND** the fake replies to AUTH with `-LOADING Redis is loading the dataset in memory`
- **AND** a command is issued
- **THEN** the command returns an error whose text is `LOADING Redis is loading the dataset in memory`
- **AND** the fake accepted one TCP connection

#### Scenario: Fake AUTH max-clients is not redialed
- **WHEN** the client is created with `New` with a non-empty password, `MaxRetries: 1`, and MinRetryBackoff off
- **AND** the fake replies to AUTH with `-ERR max number of clients reached`
- **AND** a command is issued
- **THEN** the command returns an error whose text is `ERR max number of clients reached`
- **AND** the fake accepted one TCP connection

#### Scenario: Live SELECT 99 on Redis and Dragonfly
- **WHEN** dest Redis and Dragonfly are reachable without a password
- **AND** the client is created with `New` with database `99`
- **AND** a command is issued
- **THEN** each engine returns `ERR DB index is out of range`
- **AND** the idle pool is empty
- **WHEN** those engines are unset
- **THEN** the live SELECT tests skip

#### Scenario: Live wrong password on Redis and Dragonfly with requirepass
- **WHEN** dest Redis and Dragonfly are reachable with requirepass set
- **AND** the client is created with `New` with a wrong password
- **AND** a command is issued
- **THEN** each engine returns `redis:noauth`
- **AND** the idle pool is empty
- **WHEN** those passworded engines are unset
- **THEN** the live wrong-password tests skip
