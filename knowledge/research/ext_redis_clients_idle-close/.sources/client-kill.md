---
url: https://redis.io/docs/latest/commands/client-kill/
title: CLIENT KILL
fetched: 2026-09-11
authority: official
---

CLIENT KILL closes a given client connection. Old format: CLIENT KILL addr:port matching CLIENT LIST addr.

New format: CLIENT KILL with filters (ID, TYPE, USER, ADDR, LADDR, SKIPME, MAXAGE). Multiple filters combine with logical AND. Filter form returns the number of killed clients (may be zero).

SKIPME YES (default) does not kill the calling client; SKIPME NO allows killing it too.

Due to the single-threaded nature of Redis, it is not possible to kill a client connection while it is executing a command. The client notices the close only when the next command is sent (network error).

command_flags include admin, noscript, loading, stale.
