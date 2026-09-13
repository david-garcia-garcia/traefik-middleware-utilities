## MODIFIED Requirements

### Requirement: Sleep Wake Close reclaim the flush ticker
The limiter SHALL export `Sleep`, `Wake`, and `Close` suitable for `reclaim.Hooks`. Sleep SHALL flush pending deltas then stop the flush timer. Wake SHALL start the timer when `sync_rate` is greater than zero and Sleep or Close is not waiting for an in-flight flush tick. Close SHALL run after Sleep and MUST NOT close the injected SimpleRedis client. Exact mode (`sync_rate` zero) SHALL not leak a timer. The implementation MUST NOT use `time.Tick`. The buffered flush timer MUST be `time.AfterFunc` (or another compiled stdlib waiter). It MUST NOT be an interpreted goroutine that `select`s on a ticker channel and a stop channel. While Sleep or Close is waiting for an in-flight tick, a concurrent Wake MUST NOT start a new timer, and Sleep MUST return. After Sleep returns, a later Wake SHALL start the timer when `sync_rate` is greater than zero. After Close, Wake MUST NOT start a timer.

#### Scenario: Close stops the flush timer
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Close is called (after Sleep)
- **THEN** the flush timer is stopped
- **AND** further Takes MUST NOT start a new timer
- **AND** further Wake MUST NOT start a new timer
- **AND** the SimpleRedis client remains usable by other callers

#### Scenario: Concurrent Wake does not hang Sleep
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Sleep and Wake run at the same time
- **THEN** Sleep returns
- **AND** Wake does not start a flush timer that Sleep waits for

#### Scenario: Wake after Sleep starts the timer
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Sleep has returned
- **AND** Close has not been called
- **AND** Wake is called
- **THEN** the flush timer is running

#### Scenario: Interpreted buffered Sleep returns
- **WHEN** `sync_rate` is greater than zero
- **AND** the limiter runs interpreted under Yaegi v0.16.1
- **AND** Sleep is called after buffered Takes
- **THEN** Sleep returns
- **AND** Close after that Sleep returns
