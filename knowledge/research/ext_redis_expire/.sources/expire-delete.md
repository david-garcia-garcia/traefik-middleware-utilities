---
url: https://redis.io/docs/latest/commands/expire/
title: EXPIRE — non-positive timeout deletes key
fetched: 2026-09-11
authority: official
---

Note that calling EXPIRE/PEXPIRE with a non-positive timeout or EXPIREAT/PEXPIREAT with a time in the past will result in the key being deleted rather than expired (accordingly, the emitted key event will be del, not expired).
