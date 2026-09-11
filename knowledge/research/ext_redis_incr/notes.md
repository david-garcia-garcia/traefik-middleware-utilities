# Redis INCR and INCRBY wire behavior

Official Redis string increment commands for rate-limit counters. Wire format is standard RESP2/RESP3 integer (`:`) on success and error (`-`) on failure.

## Missing key initializes to zero, then increments

`INCR` and `INCRBY` treat a missing key as `0` before applying the delta. First `INCR` on an absent key returns `1`; first `INCRBY key 5` returns `5`. ([redis.io INCR](https://redis.io/docs/latest/commands/incr/), [.sources/incr.md](.sources/incr.md); [redis.io INCRBY](https://redis.io/docs/latest/commands/incrby/), [.sources/incrby.md](.sources/incrby.md))

## Success reply is an integer

Both commands return an **integer reply** (`:` in RESP2/RESP3): the key value **after** the increment. Example from official docs: `SET mykey "10"` then `INCR mykey` → `(integer) 11`. ([redis.io INCR — Return information](https://redis.io/docs/latest/commands/incr/#return-information), [.sources/incr.md](.sources/incr.md))

## Non-integer stored value → error reply

If the key holds the wrong type or a string that cannot be parsed as a base-10 64-bit signed integer, Redis returns an **error reply** (`-ERR …`), not a miss. Official INCR/INCRBY text: "An error is returned if the key contains a value of the wrong type or contains a string that can not be represented as integer." ([redis.io INCR](https://redis.io/docs/latest/commands/incr/), [.sources/incr.md](.sources/incr.md))

## INCRBY delta zero is legal

`INCRBY` accepts any integer `increment`, including `0`. With a missing key, `INCRBY key 0` yields integer `0` (initialize to 0, add 0). With an existing integer value, delta 0 returns the current value unchanged. Confirmed in Dragonfly's Redis-compat tests at `string_family_test.cc` (`incrby ne 0` → 0; `incrby key1 0` on stored integers). ([redis.io INCRBY](https://redis.io/docs/latest/commands/incrby/), [.sources/incrby.md](.sources/incrby.md); [dragonfly@36eaa12:string_family_test.cc](https://github.com/dragonflydb/dragonfly/blob/36eaa127c2e5dd4f84724828a4c7c1412c4be90c/src/server/string_family_test.cc), [.sources/string_family_test-incr.md](.sources/string_family_test-incr.md))

## INCR and INCRBY do not refresh TTL

Increment operations alter the value in place and **leave any existing expire untouched**. Only commands that delete or overwrite the key (e.g. `DEL`, `SET`, `GETSET`, `*STORE`) clear the timeout. Official EXPIRE docs explicitly list `INCR` as an operation that leaves the timeout untouched. ([redis.io EXPIRE — Details](https://redis.io/docs/latest/commands/expire/#details), [.sources/expire-ttl-unchanged.md](.sources/expire-ttl-unchanged.md))
