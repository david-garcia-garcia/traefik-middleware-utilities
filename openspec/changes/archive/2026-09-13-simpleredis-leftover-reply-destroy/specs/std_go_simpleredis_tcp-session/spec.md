## ADDED Requirements

### Requirement: Leftover unread reply is not returned to the idle pool
When a command’s reply is one complete RESP value and unread bytes remain on that connection, the session SHALL return that decoded value to the caller and SHALL NOT return that socket to the idle pool. The session MUST NOT drain leftover bytes to resynchronise. A later command on the same client SHALL dial a new connection when the idle pool is empty after that discard. A sequential burst of Gets against a peer that writes exactly one complete reply per command SHALL still reuse one connection. When unread bytes remain before the next command is written on the same connection (AUTH then SELECT on a newly dialed socket), the session SHALL NOT write that next command on that socket.

#### Scenario: Stray extra bulk is not pooled
- **WHEN** `MaxRetries` is `-1`
- **AND** `PoolSize` is `1`
- **AND** a Get receives a complete bulk for its own key plus one extra well-formed bulk
- **THEN** that Get returns the bytes for its own key
- **AND** the idle pool is empty after that call

#### Scenario: Next command after leftover dials a new connection
- **WHEN** that leftover Get has returned
- **AND** a later Get is issued for another key against a peer that writes one complete bulk per command
- **THEN** that Get returns the bytes for its own key
- **AND** the peer accepted a new TCP connection for that later Get

#### Scenario: Compliant sequential Gets reuse one connection
- **WHEN** a client issues 25 sequential Gets against a peer that writes exactly one complete reply per command
- **THEN** the peer accepted exactly one TCP connection
