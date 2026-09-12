---
url: https://www.dragonflydb.io/docs/command-reference/server-management/client-kill
title: Redis CLIENT KILL Command (Documentation) | Dragonfly
fetched: 2026-09-11
authority: official
---

Syntax: CLIENT KILL ip:port; CLIENT KILL ADDR ip:port; CLIENT KILL LADDR ip:port; CLIENT KILL ID client-id.

Dragonfly supports ADDR (kill connections from the specified remote address), LADDR (kill connections to the specified local bind address), ID (kill a specific client by numeric ID). A single ip:port argument is equivalent to ADDR ip:port.

Some filters from Redis/Valkey (such as USER/TYPE/SKIPME) are currently not implemented in Dragonfly.

Return: Integer reply, the number of client connections that were terminated.

The command affects only connections that exist at the time of execution.
