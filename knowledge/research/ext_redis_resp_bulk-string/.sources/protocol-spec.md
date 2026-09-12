---
url: https://redis.io/docs/latest/develop/reference/protocol-spec/
title: Redis serialization protocol specification
fetched: 2026-09-11
authority: official
---

RESP2 is the default client-server protocol. A client sends an array of bulk strings; the server replies with one RESP value (exceptions: pipelining, Pub/Sub, MONITOR, RESP3 Push).

CRLF (`\r\n`) always separates RESP parts.

Bulk string encoding:
- `$` as the first byte
- unsigned decimal length in bytes
- CRLF
- the data
- a final CRLF

Example: `"hello"` → `$5\r\nhello\r\n`. Empty string → `$0\r\n\r\n`.

Null bulk string (RESP2 missing GET): `$-1\r\n`. Clients must treat that as nil, not an empty string.

The protocol describes complete values. It does not define a server command that announces a bulk length and then emits fewer payload bytes.
