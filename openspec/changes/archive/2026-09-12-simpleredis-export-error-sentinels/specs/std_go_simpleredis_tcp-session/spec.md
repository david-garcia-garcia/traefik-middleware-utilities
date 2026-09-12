## MODIFIED Requirements

### Requirement: Full pool wait returns redis:unreachable
When every live socket is checked out, a further command SHALL wait for an in-use turn. If no turn frees before the pool wait (200 milliseconds) elapses, that command SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT open another TCP connection. The wait MUST use only the Go standard library (no extra timer goroutine leak: stop the timer when a turn arrives). That timeout MUST NOT be retried. The pool-wait sentinel SHALL wrap the unreachable sentinel so `errors.Is` matches unreachable, and the retry classifier MUST still treat pool wait as not retryable (`shouldRetry` false for pool wait, true for a plain unreachable sentinel). Identity compare, or a pool-wait check before unreachable, is required; rewriting the classifier to `errors.Is` against unreachable alone MUST NOT retry pool wait.

#### Scenario: Pool wait times out
- **WHEN** all live sockets at the default `poolSize` (8) are busy
- **AND** another command is issued
- **AND** no socket becomes free within 200 milliseconds
- **THEN** that command returns `redis:unreachable`
- **AND** the fake or server observes no additional TCP connection for that command

#### Scenario: Pool wait is not retried after wrapping unreachable
- **WHEN** the error is the pool-wait sentinel
- **THEN** command retry does not retry that error
- **WHEN** the error is the plain unreachable sentinel
- **THEN** command retry does retry that error
