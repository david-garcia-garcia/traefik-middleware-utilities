---
url: https://valkey.io/commands/msetex/
title: Valkey Command · MSETEX
fetched: 2026-09-11
authority: official
---

Since: 9.1.0. Complexity O(N) where N is the number of keys to set.

Syntax: MSETEX numkeys key value [key value ...] [NX | XX] [EX seconds | PX milliseconds | EXAT unix-time-seconds | PXAT unix-time-milliseconds | KEEPTTL]

MSETEX is a combination of MSET and EXPIRE. All given keys are set at once with the specified expiration. Clients cannot observe a partial update.

EX / PX / EXAT / PXAT / KEEPTTL are mutually exclusive. NX and XX are mutually exclusive.

Example: MSETEX 2 key1 "Hello" key2 "World" EX 10 → (integer) 1; TTL of each key is 10.

NX example: second MSETEX with NX returns (integer) 0 and does not change existing keys.

RESP2/RESP3: integer 0 if no key was set (NX/XX condition); integer 1 if all keys were set.
