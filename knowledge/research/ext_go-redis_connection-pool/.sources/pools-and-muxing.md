---
url: https://redis.io/docs/latest/develop/clients/pools-and-muxing/
title: Connection pools and multiplexing
fetched: 2026-09-11
authority: official
---

Generic Redis-clients page. go-redis is listed as a pooling client (not a multiplexer).

Pooling sketch: initialize a small number of connections; "open" returns an existing in-use-marked conn; "close" returns it to the pool without closing the socket.

"If all connections in the pool are in use but the app needs more, then the client can simply open new connections as necessary." That generic sentence does not match this pin’s Get path (wait on a PoolSize turn, then ErrPoolTimeout). Follow source for go-redis v9 on this commit.
