---
url: https://www.dragonflydb.io/docs/command-reference/compatibility
title: Dragonfly API Compatibility — CLIENT KILL and CONFIG SET
fetched: 2026-09-11
authority: official
---

Verification: Dragonfly v1.40.0; Redis 8.6.4.

CLIENT KILL: Partially supported. Missing: MAXAGE, SKIPME, USER.
CLIENT LIST: Fully supported.
CLIENT ID: Fully supported.
CONFIG SET: Fully supported.
CONFIG GET: Fully supported.

"Fully supported" does not imply byte-for-byte identical behavior.
