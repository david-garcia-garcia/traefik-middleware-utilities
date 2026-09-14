## ADDED Requirements

### Requirement: Probe Config passes password and database to Init
The nested SimpleRedis Traefik plugin Config SHALL include `Password` and `Database` with empty default. `New` SHALL pass Host, Password, and Database to `Init`. Success labels on `/redis` and `/dragonfly` MUST keep host-only settings so Init receives an empty password and an empty database.

#### Scenario: Success routes stay host-only
- **WHEN** Traefik loads the plugin for `/redis` and `/dragonfly`
- **THEN** those routes Init with no password and an empty database
- **AND** a request on each route still succeeds

### Requirement: Traefik e2e proves handshake AUTH and SELECT failures
Compose SHALL add sibling Redis and Dragonfly services with requirepass. Compose MUST NOT put requirepass on the existing `redis` and `dragonfly` services that serve `/redis` and `/dragonfly`. Whoami routes SHALL Init with a wrong password against those siblings and with database `99` against the existing unpassworded `redis` and `dragonfly`. A request on a wrong-password route SHALL return HTTP 502 whose body is `redis:noauth`. A request on a database-99 route SHALL return HTTP 502 whose body contains `ERR DB index is out of range`. Those tests MUST NOT stop `whoami-a` or `whoami-b`. Eval on the success routes SHALL remain the existing Lua 5.1-safe script that lists its key in KEYS.

#### Scenario: Pester wrong password on Redis and Dragonfly
- **WHEN** a request is made on the wrong-password whoami route for Redis and for Dragonfly
- **THEN** each response is HTTP 502
- **AND** each body is `redis:noauth`
- **AND** neither test stops `whoami-a` or `whoami-b`

#### Scenario: Pester SELECT 99 on Redis and Dragonfly
- **WHEN** a request is made on the database-99 whoami route for Redis and for Dragonfly
- **THEN** each response is HTTP 502
- **AND** each body contains `ERR DB index is out of range`
- **AND** neither test stops `whoami-a` or `whoami-b`
