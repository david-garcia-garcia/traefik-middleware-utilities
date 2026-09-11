---
url: https://redis.io/docs/latest/commands/client-pause/
title: CLIENT PAUSE
fetched: 2026-09-11
authority: official
---

CLIENT PAUSE timeout [WRITE | ALL] suspends all Redis clients for timeout milliseconds. ALL is the default mode; all client commands are blocked. WRITE blocks only write commands.

The command returns OK to the caller as soon as possible so CLIENT PAUSE itself is not paused.

command_flags: admin, noscript, loading, stale. ACL: @admin @slow @dangerous @connection. Since 3.0.0.

Redis Software and Redis Cloud: unsupported.
