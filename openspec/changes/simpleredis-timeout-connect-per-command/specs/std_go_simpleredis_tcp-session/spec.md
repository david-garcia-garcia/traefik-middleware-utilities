## ADDED Requirements

### Requirement: Timeout second-connection rule is per command
The rule that a timeout MUST NOT open a second connection applies to retry of that timed-out command. It MUST NOT be read as a cap on later independent commands. After `redis:timeout`, that socket SHALL NOT return to the idle pool (the reply is still outstanding). A later independent command on the same client follows the idle-pool requirement: it SHALL dial when idle is empty. When password or database is set, that new dial SHALL run AUTH and SELECT as on any other dial. The session is not required to delay or fail-fast those later dials because earlier commands timed out.

#### Scenario: Timeout is not retried on a second connection
- **WHEN** a Get hits the I/O deadline
- **THEN** that Get returns `redis:timeout`
- **AND** command retry does not open another TCP connection for that Get

#### Scenario: Later command after timeout follows idle-empty dial
- **WHEN** that timed-out Get has returned
- **AND** the idle pool is empty
- **AND** a later Get is issued
- **THEN** that later Get dials
- **AND** if a password is set, that dial sends AUTH
