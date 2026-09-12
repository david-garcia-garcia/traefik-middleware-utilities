---
url: https://redis.io/docs/latest/commands/msetex/
title: MSETEX
fetched: 2026-09-11
authority: official
---

Since: 8.4.0. Arity -4. Complexity O(N) where N is the number of keys to set.

Syntax: MSETEX numkeys key value [key value ...] [NX | XX] [EX seconds | PX milliseconds | EXAT unix-time-seconds | PXAT unix-time-milliseconds | KEEPTTL]

numkeys: the number of keys being set. Then key/value pairs. Expiration is optional on the wire.

NX: set only if none of the keys exist. XX: set only if all of the keys exist. EX / PX / EXAT / PXAT / KEEPTTL mutually exclusive; NX / XX mutually exclusive.

RESP2/RESP3: integer 0 if none of the keys were set; integer 1 if all of the keys were set.
