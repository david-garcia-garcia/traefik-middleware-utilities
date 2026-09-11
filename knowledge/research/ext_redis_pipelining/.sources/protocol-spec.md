---
url: https://redis.io/docs/latest/develop/reference/protocol-spec/
title: Redis serialization protocol specification
fetched: 2026-09-11
authority: official
---

Request-response exceptions include: Redis requests can be pipelined — send multiple commands at once and wait for replies later.

Simple errors: first byte `-`. Clients should treat errors as exceptions; the encoded string is the error message. Examples: `-ERR unknown command 'asdf'`, `-WRONGTYPE Operation against a key holding the wrong kind of value`. Redis replies with an error when something goes wrong (wrong type, unknown command). That is a complete reply for that command.

Multiple commands and pipelining: a client can use the same connection to issue multiple commands. Multiple commands can be sent with a single write. The client can skip reading replies and continue to send commands. All the replies can be read at the end.
