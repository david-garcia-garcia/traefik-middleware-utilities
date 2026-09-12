---
url: https://redis.io/docs/latest/develop/programmability/eval-intro/
title: Introduction to Eval Scripts — script cache
fetched: 2026-09-11
authority: official
---

EVAL stores the body in a dedicated cache keyed by SHA1 digest.

SCRIPT LOAD compiles and caches without executing; EVALSHA runs by that digest.

Cache is volatile: not persisted; cleared on restart, replica failover, or SCRIPT FLUSH.

Example miss: EVALSHA fff… 0 → `(error) NOSCRIPT No matching script`.

SCRIPT EXISTS returns 1/0 per digest. SCRIPT FLUSH empties the cache (used to test client libraries).

Pipelined EVALSHA cannot handle NOSCRIPT between interleaved commands; revert to EVAL in a pipeline.
