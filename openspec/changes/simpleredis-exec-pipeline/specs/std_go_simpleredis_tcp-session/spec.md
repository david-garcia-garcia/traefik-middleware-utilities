## ADDED Requirements

### Requirement: Pipeline retry is only before a successful Flush
A pipelined batch MAY be retried once when the socket was reused from the idle pool, the error is not a timeout, and the batch has not yet been flushed to the socket. After a successful Flush, or after any reply has been read (including a first `-ERR`), the client MUST NOT retry the batch wholesale. A truncated or protocol-broken read SHALL mark the connection not reusable, return that I/O or protocol error as the batch error, and MUST NOT continue remaining replies. Single-command dead-pooled-connection retry SHALL stay as specified in the idle-pool requirement.

#### Scenario: Dead idle socket retried before Flush
- **WHEN** ExecPipeline is issued on a reused idle connection whose socket is already closed
- **AND** no Flush of that batch has succeeded
- **THEN** the client dials once more and sends the batch
- **AND** the batch error is nil when the second attempt succeeds

#### Scenario: No wholesale retry after Flush
- **WHEN** ExecPipeline has flushed the batch
- **AND** a later reply is truncated
- **THEN** the client returns an I/O or protocol error
- **AND** it does not send the batch again
- **AND** the connection is not reused
