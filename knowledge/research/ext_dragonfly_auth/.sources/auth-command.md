---
url: https://www.dragonflydb.io/docs/command-reference/server-management/auth
title: Redis AUTH Command (Dragonfly)
fetched: 2026-09-11
authority: official
---

AUTH [username] password. Omitted username implies ACL user default.

OK if the password matches; otherwise an error. requirepass also changes the ACL default user password.

Docs do not quote WRONGPASS / NOAUTH.
