## ADDED Requirements

### Requirement: IOTimeout is a stall bound on socket progress
`IOTimeout` SHALL bound quiet time on the socket, not total transfer wall time. Each kernel `Read` and `Write` on a command socket SHALL set `SetReadDeadline` or `SetWriteDeadline` to the lesser of `IOTimeout` and time remaining on the command context. Mapping MUST keep `errors.Is` against `os.ErrDeadlineExceeded` and MUST NOT type-assert `net.Error`. A compliant peer that keeps sending a bulk whose transfer outlasts one `IOTimeout` SHALL still return that value when the transfer finishes inside the overall command budget. A peer that goes silent after sending a partial bulk SHALL still return `redis:timeout` (or the caller's `Err()` when that deadline is sooner) without waiting for the overall budget when the stall window is shorter. A peer that keeps sending slowly SHALL NOT hold an in-use turn past the overall command deadline; extra turn returns SHALL stay 0. The decoder's `maxBulkLength` cap SHALL stay a parse cap, not a time-derived size.

#### Scenario: Streaming bulk that outlasts one IOTimeout returns intact
- **WHEN** a fake peer replies with a multi-megabyte bulk written in chunks whose pauses make the transfer longer than `IOTimeout`
- **AND** that transfer still finishes inside the overall command budget
- **AND** a Get is issued
- **THEN** the Get returns the payload bytes the peer sent
- **AND** the error is nil

#### Scenario: Silent mid-reply still times out on the stall bound
- **WHEN** a fake peer writes a bulk header and some payload bytes then sends nothing more
- **AND** a Get is issued with `IOTimeout` well under the overall command budget
- **THEN** the command returns `redis:timeout`
- **AND** elapsed time is less than the overall command budget

#### Scenario: Drip peer cannot pin the in-use turn past the overall budget
- **WHEN** a fake peer sends bulk payload one small chunk at a time so each chunk arrives inside `IOTimeout` but the whole transfer would exceed the overall command deadline
- **AND** a Get is issued
- **THEN** the command returns within the overall deadline plus 50 milliseconds of scheduling slack
- **AND** extra turn returns are 0
- **AND** a later Get on that client can obtain a turn

## MODIFIED Requirements

### Requirement: Whole command has an overall deadline
Each command SHALL compute one overall deadline at entry equal to `(maxRetries+1)*(DialTimeout+IOTimeout)` unless the caller's context deadline is sooner. When the library instant is sooner, the command SHALL bind it onto the caller's context (`context.WithDeadline`, same shape as `net.Dialer` / `http.Client`). Dial, AUTH, SELECT, and the command SHALL share the remaining time. AUTH and SELECT MUST NOT each add a fresh full `IOTimeout` on top of a completed TCP connect. Per-command socket I/O SHALL refresh `SetReadDeadline` and `SetWriteDeadline` to the lesser of `IOTimeout` and time remaining on each kernel `Read` and `Write`, so `IOTimeout` bounds quiet time rather than total transfer. When that overall library deadline expires, the command SHALL return `redis:timeout` and MUST NOT start another attempt. A sooner caller deadline SHALL return that context's `Err()`.

#### Scenario: Black-hole Get returns within the overall deadline
- **WHEN** a zero-Config client Gets against `203.0.113.1:6379`
- **THEN** the command returns within `(maxRetries+1)*(DialTimeout+IOTimeout)` plus 50 milliseconds of scheduling slack
- **AND** the error `Error()` text is `redis:unreachable` or `redis:timeout`

#### Scenario: Handshake stall is bounded by the overall deadline
- **WHEN** `Pass` and `Database` are set
- **AND** the peer completes the TCP handshake and never replies
- **AND** a command is issued
- **THEN** the command returns within the overall deadline
- **AND** elapsed time is less than `DialTimeout + 2×IOTimeout` times the number of attempts that would fit if each step used a fresh `IOTimeout`
