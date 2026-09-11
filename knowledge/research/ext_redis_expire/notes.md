# Redis EXPIRE and EXPIREAT wire behavior

Official Redis key-expiry commands. Success is an integer flag; missing keys are not errors.

## Syntax and success reply

- `EXPIRE key seconds [NX | XX | GT | LT]`
- `EXPIREAT key unix-time-seconds [NX | XX | GT | LT]` — same semantics as `EXPIRE`, but absolute Unix timestamp (seconds since 1970-01-01). ([redis.io EXPIREAT](https://redis.io/docs/latest/commands/expireat/), [.sources/expireat.md](.sources/expireat.md))

Both return an **integer reply** (`:` in RESP2/RESP3):

| Reply | Meaning |
|-------|---------|
| `:1` | Timeout was set |
| `:0` | Timeout was **not** set (key missing, or conditional flag skipped the update) |

([redis.io EXPIRE — Return information](https://redis.io/docs/latest/commands/expire/#return-information), [.sources/expire-return.md](.sources/expire-return.md); [redis.io EXPIREAT — Return information](https://redis.io/docs/latest/commands/expireat/#return-information), [.sources/expireat.md](.sources/expireat.md))

## Missing key returns `:0`, not a miss

When the key does not exist, return is integer `0` — "the timeout was not set; for example, the key doesn't exist." This is a normal integer reply, not a null bulk or `-ERR`. ([redis.io EXPIRE — Return information](https://redis.io/docs/latest/commands/expire/#return-information), [.sources/expire-return.md](.sources/expire-return.md))

## Non-positive EXPIRE / past EXPIREAT deletes the key

Calling `EXPIRE`/`PEXPIRE` with a **non-positive** timeout, or `EXPIREAT`/`PEXPIREAT` with a timestamp **in the past**, **deletes the key** rather than attaching a zero-length TTL. Official text: "will result in the key being deleted rather than expired (accordingly, the emitted key event will be `del`, not `expired`)." ([redis.io EXPIRE](https://redis.io/docs/latest/commands/expire/), [.sources/expire-delete.md](.sources/expire-delete.md); [redis.io EXPIREAT](https://redis.io/docs/latest/commands/expireat/), [.sources/expireat.md](.sources/expireat.md))

This finding records **server** behavior only. Client retry or ignore policy for `:0` vs delete-on-zero is out of scope here.
