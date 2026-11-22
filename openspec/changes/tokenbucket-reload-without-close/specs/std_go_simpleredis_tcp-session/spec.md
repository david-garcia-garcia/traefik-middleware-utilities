# Spec Delta

## ADDED Requirements

### Requirement: Dropped clients without Close release sockets after GC
When a SimpleRedis client has run at least one command and is then dropped without Close, after garbage collection no TCP sockets from those clients SHALL remain open. New MUST NOT start a background goroutine. This requirement MUST NOT change idle-until-Close for a still-reachable client.

#### Scenario: Fifty dropped limiter clients leave no proxied sockets
- **WHEN** tests are not `-short` and a tokenbucket live engine address is set
- **AND** fifty times a new SimpleRedis client and Redis limiter are built through a counting TCP proxy, Allow is called once, and the client is dropped without Close
- **THEN** after GC the proxy open count is 0
- **AND** leaked goroutines excluding the proxy's own fail only when greater than 2
- **AND** the proof MUST NOT assert finalizers or netFD
- **AND** the proxy MUST NOT hold writes
