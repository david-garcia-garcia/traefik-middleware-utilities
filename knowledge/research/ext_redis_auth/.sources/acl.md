---
url: https://redis.io/docs/latest/operate/oss_and_stack/management/security/acl/
title: ACL
fetched: 2026-11-21
authority: official
---

"The Redis AUTH command was extended in Redis 6, so now it is possible to use it in the two-arguments form:" `AUTH <username> <password>`.

`nopass`: "the user is flagged as requiring no password: it means that every password will work against this user."

`><password>`: "Add this password to the list of valid passwords for the user. For example `>mypass` will add "mypass" to the list of valid passwords." Example: `ACL SETUSER alice on >p1pp0 ~cached:* +get`.
