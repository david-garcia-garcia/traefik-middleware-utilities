---
url: https://www.dragonflydb.io/docs/command-reference/generic/script-exists
title: Redis SCRIPT EXISTS Command (Documentation) | Dragonfly
fetched: 2026-09-11
authority: official
---

Syntax: SCRIPT EXISTS sha1 [sha1 ...]

Returns an array of integers, 1 if that SHA1 is in the script cache, 0 otherwise.

Useful before pipelining so EVALSHA does not miss. This product does not pipeline Eval; Pester uses EXISTS as the live cache probe.
