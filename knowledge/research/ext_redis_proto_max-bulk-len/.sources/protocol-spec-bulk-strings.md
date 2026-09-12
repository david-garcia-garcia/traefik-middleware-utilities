---
url: https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings
title: Redis serialization protocol specification — Bulk strings
fetched: 2026-09-12
authority: official
---

A bulk string represents a single binary string.

The string can be of any size, but by default, Redis limits it to 512 MB (see the `proto-max-bulk-len` configuration directive).

RESP encoding: `$` + unsigned decimal length in bytes + CRLF + data + CRLF.
