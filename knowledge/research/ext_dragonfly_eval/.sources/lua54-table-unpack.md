---
url: https://www.lua.org/manual/5.4/manual.html#pdf-table.unpack
title: Lua 5.4 — table.unpack
fetched: 2026-09-11
authority: official
---

table.unpack (list [, i [, j]])

Returns the elements from the given list. Equivalent to return list[i], list[i+1], ···, list[j]. Default i is 1 and j is #list.

In Lua 5.4 the function lives on the table library as table.unpack. There is no global unpack in this manual section. Dragonfly EVAL is Lua 5.4.4.
