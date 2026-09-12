---
url: https://github.com/redis/redis/blob/7.2.4/src/eval.c
title: eval.c sha1hex and EVALSHA miss
fetched: 2026-09-11
authority: source
ref: github.com/redis/redis@7.2.4:src/eval.c
---

sha1hex: SHA1 of script bytes, written as 40 lowercase hex chars plus NUL (cset 0123456789abcdef).

evalShaCommand: if argv SHA length is not 40, addReplyErrorObject(c, shared.noscripterr) and return.

On EVALSHA when the Lua function is missing: addReplyErrorObject(c, shared.noscripterr).
