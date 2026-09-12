## MODIFIED Requirements

### Requirement: Traefik request SET plus GET sets a response header
A request through the nested SimpleRedis Traefik plugin SHALL SET a key to that request’s unique token (not a shared constant such as `"ok"`) and GET it back, then set a response header from that GET so Pester can assert the round-trip. MGet of that same key SHALL return the same token as Get. The same request SHALL also call Del, Incr, IncrBy, Expire, ExpireAt, and Eval against that backend, and set one response header per verb. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Existing whoami routes `/redis` and `/dragonfly` SHALL stay; this change MUST NOT add compose services, routes, or image pin changes. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS (INCRBY plus EXPIREAT when the key is new). Pester SHALL run Get/MGet own-value assertions on both `/redis` (Redis) and `/dragonfly` (Dragonfly).

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on the plugin’s whoami route `/redis`
- **THEN** the response includes a header whose value is the unique token GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval that show those commands succeeded
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts Get and MGet own-value on Redis and Dragonfly
- **WHEN** two overlapping requests are made on `/redis`
- **AND** two overlapping requests are made on `/dragonfly`
- **THEN** each response’s Get header equals the unique token that request Set
- **AND** that response’s MGet header equals its Get header
- **AND** the two requests on the same route have distinct Get header values
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`
