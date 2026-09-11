---
url: https://github.com/dragonflydb/dragonfly/blob/main/docs/differences.md
ref: dragonflydb/dragonfly@main:docs/differences.md
title: Differences with Redis — EXPIRE flags
fetched: 2026-09-11
authority: official
---

EXPIRE, PEXPIRE, EXPIREAT and PEXPIREAT accept NX together with GT or LT: the expiry is set when the key has none, otherwise GT or LT alone decides. Redis rejects these combinations as incompatible.
