---
url: https://www.dragonflydb.io/docs/command-reference/compatibility
title: Dragonfly API Compatibility
fetched: 2026-09-11
authority: official
---

The table tracks command-surface compatibility. "Fully supported" does not imply byte-for-byte identical behavior.

List / BLPOP: Fully supported.
List / BRPOP: Fully supported.
List / BLMOVE: Fully supported.
List / BLMPOP: Fully supported.

Connection / CLIENT PAUSE: Fully supported.
Connection / CLIENT UNPAUSE: Fully supported.

Scripting / SCRIPT DEBUG: Unsupported.

No Redis DEBUG (server) row in the matrix. JSON.DEBUG is a JSON module command.

Verification: Dragonfly v1.40.0; Redis 8.6.4.
