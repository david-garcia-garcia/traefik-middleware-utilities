---
url: https://redis.io/docs/latest/commands/auth/
title: AUTH
fetched: 2026-09-11
authority: official
---

AUTH authenticates the connection when requirepass is set or Redis 6+ ACLs are in use.

Single-argument form: AUTH password — authenticates as user default (requirepass compatibility).

Two-argument form (6.0+): AUTH username password.

Success: simple string OK. Otherwise an error if the password or username/password pair is invalid.

Docs do not quote WRONGPASS / NOAUTH / the nopass AUTH text; those live in source and on the dest image.
