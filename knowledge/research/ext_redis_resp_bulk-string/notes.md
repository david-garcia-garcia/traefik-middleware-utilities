# RESP2 bulk strings

Wire shape a SimpleRedis client must read for GET/MGET payloads, and what a truncated-reply test is allowed to fake.

## Complete bulk

A RESP2 bulk string is `$<length>\r\n<data>\r\n`. Length is the payload byte count, not including the final CRLF. The client therefore reads `length+2` bytes after the header line. `"hello"` is `$5\r\nhello\r\n`. ([redis.io protocol spec — Bulk strings](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings), [.sources/protocol-spec.md](.sources/protocol-spec.md))

## Null bulk

A missing GET key is the null bulk `$-1\r\n`, not a zero-length bulk. That is a complete reply. ([redis.io protocol spec — Null bulk strings](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-bulk-strings), [.sources/protocol-spec.md](.sources/protocol-spec.md))

## Truncation is not a Redis command

RESP is a request-response protocol: the server emits one complete RESP value per command (pipelining, Pub/Sub, MONITOR, and RESP3 Push are the documented exceptions). The spec does not define a command that announces `$100` and then writes fewer than 102 payload+CRLF bytes. ([redis.io protocol spec — Request-Response model](https://redis.io/docs/latest/develop/reference/protocol-spec/#request-response-model), [.sources/protocol-spec.md](.sources/protocol-spec.md))

`authority: inference` — a mid-stream close after a lying length is a transport failure (crash, proxy, `CLIENT KILL`), not something compose Redis or Dragonfly can be told to emit. Truncated-payload tests belong on a fake that writes raw bytes and closes.
