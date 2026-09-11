## MODIFIED Requirements

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most eight. A sequential burst of Gets on one client SHALL reuse one connection. After overlapping commands that exceed eight, idle SHALL be at most eight and sockets beyond that cap SHALL be closed. The session MUST NOT refuse a dial solely because more than eight commands are in flight. An idle connection older than thirty seconds SHALL not be reused; the next command SHALL dial a new one. A dead pooled connection SHALL be retried once unless the error is a timeout. After overlapping requests on `/redis` and on `/dragonfly`, each backend SHALL have at most eight idle client connections.

#### Scenario: Sequential gets reuse one connection
- **WHEN** twenty-five Gets run one after another against a live fake Redis
- **THEN** the fake observes one TCP connection

#### Scenario: Idle cap after overlapping commands
- **WHEN** more than eight concurrent commands run against a live fake Redis
- **THEN** after those commands finish, idle is at most eight
- **AND** sockets beyond that cap are closed

#### Scenario: Live overlap on Redis leaves at most eight idle sockets
- **WHEN** sixteen overlapping requests are made on `/redis`
- **AND** those requests finish
- **THEN** Redis has at most eight idle client connections
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Live overlap on Dragonfly leaves at most eight idle sockets
- **WHEN** sixteen overlapping requests are made on `/dragonfly`
- **AND** those requests finish
- **THEN** Dragonfly has at most eight idle client connections
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

### Requirement: Close drains the pool and blocks redial
`Close` SHALL close idle pooled connections and mark the client closed. After `Close`, `Get`, `MGet`, `Set`, and `Del` SHALL return `redis:unreachable` and MUST NOT dial. `Close` SHALL be idempotent. In-flight commands MAY finish; their sockets SHALL be closed on release.

#### Scenario: Close drains idle and does not redial
- **WHEN** a client has an idle pooled connection
- **AND** `Close` is called
- **AND** a later Get is issued
- **THEN** that Get returns `redis:unreachable`
- **AND** no new TCP connection is opened

#### Scenario: Close during an in-flight command closes the socket on release
- **WHEN** a command is in flight
- **AND** `Close` is called
- **AND** that command then finishes
- **THEN** idle is empty
- **AND** that command's socket is closed
- **AND** a later Get returns `redis:unreachable`
- **AND** no new TCP connection is opened
