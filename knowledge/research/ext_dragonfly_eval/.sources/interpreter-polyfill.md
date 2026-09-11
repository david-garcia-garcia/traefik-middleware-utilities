---
url: https://github.com/dragonflydb/dragonfly/blob/36eaa127c2e5dd4f84724828a4c7c1412c4be90c/src/core/interpreter_polyfill.h
ref: dragonflydb/dragonfly@36eaa127:src/core/interpreter_polyfill.h
title: interpreter_polyfill.h
fetched: 2026-09-11
authority: source
---

Header: implementations of deprecated, removed or renamed lua functions for Lua 5.4.

register_polyfills adds to table:
- table.getn (polyfill via lua_len)
- table.setn (errors "setn is obsolete")
- table.foreach
- table.foreachi

No table.maxn polyfill registered.

interpreter_test.cc: table.getn{1,2,3} works; table.setn throws error.
