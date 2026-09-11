---
url: https://github.com/redis/redis/blob/7.2.4/src/db.c
title: db.c selectCommand
fetched: 2026-09-11
authority: source
ref: redis/redis@7.2.4:src/db.c
---

selectCommand: parse integer index; cluster + id != 0 → "SELECT is not allowed in cluster mode"; selectDb C_ERR → addReplyError "DB index is out of range"; else OK.

Live redis:7-alpine 7.4.10 (2026-09-11): databases=16, SELECT 15 OK, SELECT 99 → ERR DB index is out of range.
