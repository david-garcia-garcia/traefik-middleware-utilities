# Redis pipelining wire behavior

Pipelining is a client/server RESP pattern: the client writes several commands without waiting for each reply, then reads one reply per command in order. It is not a Redis command and is not MULTI/EXEC. Redis has supported it since early versions. ([redis.io Pipelining](https://redis.io/docs/latest/develop/using-commands/pipelining/), [.sources/pipelining.md](.sources/pipelining.md); [redis.io Protocol spec — Multiple commands and pipelining](https://redis.io/docs/latest/develop/reference/protocol-spec/#multiple-commands-and-pipelining), [.sources/protocol-spec.md](.sources/protocol-spec.md))

## One write of N commands, then N replies in order

The protocol allows a client to send multiple commands with a single write and skip reading until the end. "All the replies can be read at the end." Official netcat example: three `PING` frames, then three `+PONG` replies. Sequential INCR becomes four client writes of INCR followed by four integer replies. ([protocol spec](https://redis.io/docs/latest/develop/reference/protocol-spec/#multiple-commands-and-pipelining), [.sources/protocol-spec.md](.sources/protocol-spec.md); [Pipelining](https://redis.io/docs/latest/develop/using-commands/pipelining/), [.sources/pipelining.md](.sources/pipelining.md))

hiredis documents the same split: `redisAppendCommand` queues without sending; `redisGetReply` flushes the buffer on the first call, then the client must call `redisGetReply` once per queued command. ([hiredis Pipelines and transactions](https://redis.io/docs/latest/develop/clients/hiredis/transpipe/), [.sources/hiredis-transpipe.md](.sources/hiredis-transpipe.md))

## Mixed verbs are legal, including EVAL

The protocol does not restrict which commands may share a pipeline. Official pipelining examples mix the idea of "multiple commands" (PING, INCR). hiredis's worked example pipelines SET then GET. Redis also states that sending `EVAL` or `EVALSHA` in a pipeline "is entirely possible". SCRIPT LOAD is a separate concern (EVALSHA). ([Pipelining — vs Scripting](https://redis.io/docs/latest/develop/using-commands/pipelining/), [.sources/pipelining.md](.sources/pipelining.md); [hiredis example](https://redis.io/docs/latest/develop/clients/hiredis/transpipe/), [.sources/hiredis-transpipe.md](.sources/hiredis-transpipe.md))

## `-` error replies do not drop the remaining replies

A simple error (`-ERR …`, `-WRONGTYPE …`) is a complete RESP value for that command. The client "should raise an exception when it receives an Error reply" — for that reply. The pipeline rule is still one reply per command: hiredis iterates `i in 0..N-1` and checks `REDIS_REPLY_ERROR` per slot. Nothing in the protocol says the server omits later replies because an earlier command returned `-`. ([protocol spec — Simple errors](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-errors), [.sources/protocol-spec.md](.sources/protocol-spec.md); [hiredis loop](https://redis.io/docs/latest/develop/clients/hiredis/transpipe/), [.sources/hiredis-transpipe.md](.sources/hiredis-transpipe.md))

## I/O or a truncated stream is not a `-` reply

If the socket fails or the stream stops mid-value, there is no remaining well-formed reply to read. The protocol's "read all replies at the end" assumes the connection stayed up. A client that does not consume the reply stream cannot reuse that connection for further commands (hiredis: further `redisGetReply` on an unread pipeline leaves the context unusable). That is a connection-level failure, not an element `-ERR`. ([protocol spec — pipelining](https://redis.io/docs/latest/develop/reference/protocol-spec/#multiple-commands-and-pipelining), [.sources/protocol-spec.md](.sources/protocol-spec.md); [hiredis](https://redis.io/docs/latest/develop/clients/hiredis/transpipe/), [.sources/hiredis-transpipe.md](.sources/hiredis-transpipe.md))

## Server memory: batch, then read, then batch again

While the client pipelines, the server queues replies. Official guidance: send a reasonable batch (example: 10k commands), read the replies, then send the next batch, so queued-reply memory stays bounded. Pipelining is not atomic and cannot replace a script when the next write depends on the previous read. ([Pipelining — IMPORTANT NOTE](https://redis.io/docs/latest/develop/using-commands/pipelining/), [.sources/pipelining.md](.sources/pipelining.md))
