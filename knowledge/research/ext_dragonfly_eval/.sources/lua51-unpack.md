---
url: https://www.lua.org/manual/5.1/manual.html#pdf-unpack
title: Lua 5.1 — unpack
fetched: 2026-09-11
authority: official
---

unpack (list [, i [, j]])

Returns the elements from the given table. Equivalent to return list[i], list[i+1], ···, list[j] except that that form can be written only for a fixed number of elements. Default i is 1 and j is the length of the list (length operator).

This is a global in Lua 5.1. Redis EVAL embeds Lua 5.1.
