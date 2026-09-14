---
url: https://github.com/redis/redis/blob/7.2.4/src/acl.c
title: acl.c AUTH helpers
fetched: 2026-09-11
authority: source
ref: redis/redis@7.2.4:src/acl.c
---

addAuthErrReply: default error "-WRONGPASS invalid username-password pair or user is disabled."

authCommand (argc==2): if DefaultUser has USER_FLAG_NOPASS, addReplyError "AUTH called without any password configured for the default user. Are you sure your configuration is correct?" and return. Else username is default.

Live redis:7-alpine (7.4.10, 2026-09-11) matched both strings.
