---
url: https://redis.io/docs/latest/develop/programmability/eval-intro/
title: Scripting with Lua
fetched: 2026-09-11
authority: official
---

Redis guarantees the script's atomic execution. While executing the script, all server activities are blocked during its entire runtime. These semantics mean that all of the script's effects either have yet to happen or had already happened.

The potential downside: executing slow scripts is not a good idea. All other clients are blocked and can't execute any command while it is running.

Default engine is Lua 5.1.
