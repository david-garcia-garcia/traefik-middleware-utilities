---
url: https://redis.io/docs/latest/develop/reference/clients/
title: Redis client handling — Client Timeouts and CLIENT LIST
fetched: 2026-09-11
authority: official
---

By default recent versions of Redis don't close the connection with the client if the client is idle for many seconds: the connection will remain open forever.

A timeout can be configured so that if the client is idle for more than the specified number of seconds, the client connection will be closed. Configure via redis.conf or CONFIG SET timeout.

The timeout only applies to normal clients and it does not apply to Pub/Sub clients.

Timeouts are not to be considered very precise: Redis avoids setting timer events or running O(N) algorithms in order to check idle clients, so the check is performed incrementally. A timeout set to 10 seconds may close after 12 seconds if many clients are connected.

CLIENT LIST fields include addr (client IP and remote port), fd, name, age, idle, flags, cmd (last executed command). Once you have the list, close a connection with CLIENT KILL specifying the client address.
