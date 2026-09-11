---
url: https://www.dragonflydb.io/blog/batch-operations-in-dragonfly
title: Batch Operations in Dragonfly: Pipelining, Transactions, and Lua Scripting
fetched: 2026-09-11
authority: comment
---

Vendor blog (Joe Zhou, 2024-06-27). Weaker than flags/docs; used only where it states product behavior not on the flags page.

Pipelining is supported: send a series of commands without waiting for each reply; client receives all responses. Example: redis-cli --pipe and go-redis Pipelined mixed PFADD commands.

Pipelining is not atomic and has no isolation; not all operations may succeed together. MULTI/EXEC is the atomic batch; WATCH for isolation. Lua scripting is a separate atomic path.

Dragonfly is presented as a Redis drop-in for pipelining/transactions/Lua. Extra vs Redis: parallel command execution when commands are pipelined (multi-thread). pipeline_squash is an experimental parallelization of a single large pipeline.

Do not treat this page as the owner of queue/buffer numeric defaults (those are on the flags page).
