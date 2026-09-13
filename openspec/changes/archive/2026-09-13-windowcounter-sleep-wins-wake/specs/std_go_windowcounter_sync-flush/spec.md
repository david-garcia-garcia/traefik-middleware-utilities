## MODIFIED Requirements

### Requirement: Sleep Wake Close reclaim the flush ticker
The limiter SHALL export `Sleep`, `Wake`, and `Close` suitable for `reclaim.Hooks`. Sleep SHALL flush pending deltas then stop the ticker. Wake SHALL start the ticker when `sync_rate` is greater than zero and Sleep or Close is not waiting for the flush loop to exit. Close SHALL run after Sleep and MUST NOT close the injected SimpleRedis client. Exact mode (`sync_rate` zero) SHALL not leak a ticker. The implementation MUST NOT use `time.Tick`. While Sleep or Close is waiting for the flush loop, a concurrent Wake MUST NOT start a new ticker, and Sleep MUST return. After Sleep returns, a later Wake SHALL start the ticker when `sync_rate` is greater than zero. After Close, Wake MUST NOT start a ticker.

#### Scenario: Close stops the flush goroutine
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Close is called (after Sleep)
- **THEN** the flush goroutine has exited
- **AND** further Takes MUST NOT start a new ticker
- **AND** further Wake MUST NOT start a new ticker
- **AND** the SimpleRedis client remains usable by other callers

#### Scenario: Concurrent Wake does not hang Sleep
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Sleep and Wake run at the same time
- **THEN** Sleep returns
- **AND** Wake does not start a flush loop that Sleep waits for

#### Scenario: Wake after Sleep starts the ticker
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Sleep has returned
- **AND** Close has not been called
- **AND** Wake is called
- **THEN** the flush ticker is running
