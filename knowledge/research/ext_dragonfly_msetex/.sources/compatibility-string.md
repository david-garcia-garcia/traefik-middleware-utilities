---
url: https://www.dragonflydb.io/docs/command-reference/compatibility
title: Dragonfly API Compatibility
fetched: 2026-09-11
authority: official
---

Verification line on the page: Dragonfly v1.40.0; Redis 8.6.4.

String family rows present: APPEND, DECR, DECRBY, DIGEST, GET, GETEX, GETDEL, GETRANGE, GETSET, INCR, INCRBY, INCRBYFLOAT, LCS (Unsupported), MGET, MSET, MSETNX, PSETEX, SET (Partially supported — missing IFDEQ, IFDNE, IFEQ, IFNE), SETEX, SETNX, SETRANGE, STRLEN, SUBSTR.

MSETEX does not appear in the String family (or elsewhere on the matrix).

Scripting family includes EVAL, EVALSHA, SCRIPT LOAD/EXISTS/FLUSH (FLUSH partial).

This product pins `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` (see `ext_dragonfly_container-image`).
