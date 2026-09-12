---
url: https://www.dragonflydb.io/docs/command-reference/generic/evalsha
title: Redis EVALSHA Command (Documentation) | Dragonfly
fetched: 2026-09-11
authority: official
---

Syntax: EVALSHA sha1 numkeys [key [key ...]] [arg [arg ...]]

Evaluate a script from the server's cache by its SHA1 digest.

The server caches scripts by using the SCRIPT LOAD command. The command is otherwise identical to EVAL.
