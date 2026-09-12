---
url: https://redis.io/docs/latest/develop/reference/protocol-spec/
title: RESP protocol spec — null bulk strings and null arrays
fetched: 2026-09-11
authority: official
---

RESP2 has no dedicated null type. Null is encoded as predetermined bulk-string and array forms.

Null bulk string: `$-1\r\n`. GET of a missing key returns this. A client should return a nil object, not an empty string.

Empty array: `*0\r\n`.

Null array: `*-1\r\n` (array length -1). BLPOP timeout returns a null array. A client should return a null object rather than an empty array, so timeout is distinct from an empty list.

RESP3 adds `_` (`_\r\n`) as a dedicated null. The spec calls the RESP2 dual null-bulk / null-array forms a historical redundancy.
