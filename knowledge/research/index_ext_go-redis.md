# ext / go-redis

## go-redis connection pool
priority: normal
local: ext_go-redis_connection-pool/
description: How go-redis caps live vs idle sockets, waits on a pool turn, and what PoolTimeout returns.

## Reader.readLine
priority: normal
local: ext_go-redis_proto_readline/
description: How go-redis proto.Reader.readLine uses ReadSlice and handles bufio.ErrBufferFull.

## proto.Reader bulk and array size
priority: normal
local: ext_go-redis_proto_reader-limit/
description: Whether go-redis proto.Reader caps bulk-string or array allocations taken from a reply header.
