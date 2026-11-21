## MODIFIED Requirements

### Requirement: Allow maps admit, wait, and deny
`Allow(ctx, key)` SHALL consume one token for the opaque `key` and return a `Status` and an error. `Memory.Allow` and `Redis.Allow` SHALL take the same arguments and return the same `Status` type. A caller with no deadline SHALL pass `context.Background()`. The wait is the time until tokens reach zero after this consume, in microseconds; `Status` SHALL report it as `Delay` when allowed and as `RetryAfter` when denied (`std_go_tokenbucket_status`). `Allowed` SHALL be false with `Delay` and `RetryAfter` zero when the store cannot reserve (burst cannot cover this consume). `Allowed` SHALL be false when the wait in microseconds is greater than `maxDelay` truncated to whole microseconds (the store SHALL refund that consume). `Allowed` SHALL be true when that wait in microseconds is at most `maxDelay` truncated to whole microseconds. The `Delay` and `RetryAfter` durations MUST NOT decide `Allowed`. When the error is non-nil, the `Status` SHALL be the zero value. The library MUST NOT sleep. Prefixing of `key` is the caller's job.

#### Scenario: Burst after idle then next delayed or denied
- **WHEN** a key has been idle long enough to fill to `burst`
- **AND** Allow is called `burst` times for that key with `maxDelay` large enough to admit those consumes
- **THEN** each of those Allows returns `Allowed` true
- **WHEN** Allow is called once more for that key
- **THEN** that Allow returns `Allowed` false with a positive `RetryAfter` if the wait in microseconds is greater than `maxDelay` truncated to whole microseconds, or `Allowed` true with a positive `Delay` if that wait in microseconds is at most `maxDelay` truncated to whole microseconds

#### Scenario: Refund when wait exceeds maxDelay
- **WHEN** tokens after consume are negative and the wait in microseconds is greater than `maxDelay` truncated to whole microseconds
- **THEN** Allow refunds that consume
- **AND** returns `Allowed` false
- **AND** the next Allow on that key sees the refunded tokens (not a second consume stacked on the denied one)

#### Scenario: Refund and admit share microseconds
- **WHEN** the wait in microseconds is greater than `maxDelay` truncated to whole microseconds
- **AND** that wait converted to a duration is still at most `maxDelay`
- **THEN** Allow refunds that consume
- **AND** returns `Allowed` false
- **AND** a following Allow on that key at the same instant is not a stacked consume

#### Scenario: Caller owns the key
- **WHEN** Allow is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

#### Scenario: Error returns the zero status
- **WHEN** Allow returns a non-nil error (cancelled context, Redis error, or a malformed Eval reply)
- **THEN** the returned `Status` is the zero value
- **AND** its `Allowed` is false

## ADDED Requirements

### Requirement: Eval tokens that is not a finite number is not a consume
When the Redis store is used and Eval returns three fields, Allow SHALL read the third field as the tokens left after this Allow. A tokens field that is not a finite number, including `nan`, `+Inf`, `-Inf`, `inf`, or text that does not parse, SHALL return `tokenbucket: eval tokens is not a number`. Allow MUST NOT return a `Status` with a nil error in that case. The Eval script MUST NOT change to produce this field.

#### Scenario: Non-finite tokens is tokens-is-not-a-number
- **WHEN** Eval returns three fields whose wait is a finite number and whose tokens is `nan`, `+Inf`, `-Inf`, `inf`, or `abc`
- **THEN** Allow returns `tokenbucket: eval tokens is not a number`
- **AND** the returned `Status` is the zero value

#### Scenario: Tokens text from either engine parses
- **WHEN** Eval returns a tokens field written as `4`, `4.0`, `-1.0`, `0.3`, or `1e+15`
- **THEN** Allow returns a nil error
- **AND** the `Status` is built from that tokens value
