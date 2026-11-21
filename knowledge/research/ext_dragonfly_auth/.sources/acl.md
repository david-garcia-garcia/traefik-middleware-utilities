---
url: https://www.dragonflydb.io/docs/managing-dragonfly/acl
title: Access Control Lists (ACL)
fetched: 2026-09-11
authority: official
---

Default user can AUTH using any password and run any command unless default is OFF or given a password.

--requirepass changes the default user password. Incoming connections then must AUTH.

Added 2026-11-21 from the same page: "ACL SETUSER Mike >mypassword, creates the user `Mike` with pass `mypassword`." "Users can use `AUTH <username> <password>` to authorize their connection with a given user." "By default, all users (except `default`) are `OFF` (unless you explicitly grant them `ON`)."
