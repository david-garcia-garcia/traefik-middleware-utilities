# RESP2 null array

Official RESP2 encoding of a **null array** (`*-1\r\n`) versus a null bulk string and an empty array. SimpleRedis `readReply` currently treats `count < 0` as `redis:issue?`. ([redis.io RESP protocol spec](https://redis.io/docs/latest/develop/reference/protocol-spec/), [.sources/protocol-spec-null-array.md](.sources/protocol-spec-null-array.md))

## Encoding

RESP2 has no dedicated null type. Null is encoded as predetermined bulk-string or array forms. ([protocol spec — Null bulk strings / Null arrays](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-arrays), [.sources/protocol-spec-null-array.md](.sources/protocol-spec-null-array.md))

| Form | Wire | Meaning |
|------|------|---------|
| Null bulk string | `$-1\r\n` | Missing value (e.g. `GET` of a missing key). Client should return nil, not empty string. |
| Empty array | `*0\r\n` | Zero elements. Distinct from null. |
| Null array | `*-1\r\n` | Null object, not an empty array. Client should return null, not `[]`. |
| RESP3 null | `_\r\n` | Dedicated null (not RESP2). |

## Commands that return a null array

- **BLPOP** (and other blocking list pops) on timeout: protocol spec names this as the example of a null array. ([protocol spec — Null arrays](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-arrays), [.sources/protocol-spec-null-array.md](.sources/protocol-spec-null-array.md))
- **EXEC** when a `WATCH`ed key was touched: RESP2 **Nil reply** (the transaction was aborted). ([redis.io EXEC — Return information](https://redis.io/docs/latest/commands/exec/#return-information), [.sources/exec-nil-reply.md](.sources/exec-nil-reply.md))

SimpleRedis has no `BLPOP`, `MULTI`/`EXEC`, or other blocking verbs. `Eval` Lua `false` maps to a **null bulk**, not a null array (`ext_redis_eval`).

## Client contract

When Redis replies with a null array, the client should return a null object rather than an empty array, so a timeout/abort is not confused with an empty collection. ([protocol spec — Null arrays](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-arrays), [.sources/protocol-spec-null-array.md](.sources/protocol-spec-null-array.md))

Null bulk (`$-1`) is a different form: this product already maps that to `redis:miss` in `readBulk`.
