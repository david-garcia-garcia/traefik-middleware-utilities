## MODIFIED Requirements

### Requirement: Allow maps admit, wait, and deny
`Allow(ctx, key)` SHALL consume one token for the opaque `key` and return `allowed` plus a wait duration. `Memory.Allow` and `Redis.Allow` SHALL take the same arguments. A caller with no deadline SHALL pass `context.Background()`. `allowed` SHALL be false and wait zero when the store cannot reserve (burst cannot cover this consume). `allowed` SHALL be false when the wait until tokens reach zero, in microseconds, is greater than `maxDelay` truncated to whole microseconds (the store SHALL refund that consume). `allowed` SHALL be true when that wait in microseconds is at most `maxDelay` truncated to whole microseconds. The wait duration return MUST NOT decide `allowed`. The library MUST NOT sleep. Prefixing of `key` is the caller's job.

#### Scenario: Burst after idle then next delayed or denied
- **WHEN** a key has been idle long enough to fill to `burst`
- **AND** Allow is called `burst` times for that key with `maxDelay` large enough to admit those consumes
- **THEN** each of those Allows returns allowed true
- **WHEN** Allow is called once more for that key
- **THEN** that Allow returns allowed false if the wait in microseconds is greater than `maxDelay` truncated to whole microseconds, or allowed true with a positive wait if that wait in microseconds is at most `maxDelay` truncated to whole microseconds

#### Scenario: Refund when wait exceeds maxDelay
- **WHEN** tokens after consume are negative and the wait in microseconds is greater than `maxDelay` truncated to whole microseconds
- **THEN** Allow refunds that consume
- **AND** returns allowed false
- **AND** the next Allow on that key sees the refunded tokens (not a second consume stacked on the denied one)

#### Scenario: Refund and admit share microseconds
- **WHEN** the wait in microseconds is greater than `maxDelay` truncated to whole microseconds
- **AND** that wait converted to a duration is still at most `maxDelay`
- **THEN** Allow refunds that consume
- **AND** returns allowed false
- **AND** a following Allow on that key at the same instant is not a stacked consume

#### Scenario: Caller owns the key
- **WHEN** Allow is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key
