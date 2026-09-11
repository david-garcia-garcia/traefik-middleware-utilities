---
url: https://www.dragonflydb.io/docs/managing-dragonfly/scripting
title: Scripting with Lua
fetched: 2026-09-11
authority: official
---

Dragonfly allows users to execute scripts written in Lua. Interface compatible with Redis.
Dragonfly uses Lua version 5.4.

Undeclared key access error: "script tried accessing undeclared key".
Flag allow-undeclared-keys disables the restriction; when enabled Dragonfly must stop all other operations while the script runs (unpredictability, atomicity and multithreading don't mix well).

Flags via:
- Script shebang: --!df flags=allow-undeclared-keys
- Server default: --default_lua_flags=allow-undeclared-keys
- SCRIPT FLAGS sha1 allow-undeclared-keys

Sandbox: load() text-only; protected rawset/setmetatable/getmetatable on _G; dragonfly.randstr() size limits.
