---
url: https://github.com/redis/redis/pull/9107
title: Make unknown command error message more friendly
fetched: 2026-09-11
authority: source
---

Redis unknown-command errors are `-ERR` payloads whose text starts with `ERR unknown command`.

Old form (quoted with backticks): `ERR unknown command `testcommand`, with args beginning with:` and, when args exist, those args listed.

Newer form in that PR: no-arg → `ERR unknown command 'testcommand'`; with args → `ERR unknown command `testcommand`, with args beginning with: `arg1`, `arg2``.

SimpleRedis `replyError` returns `errors.New` of the payload without the leading `-`. A detector that treats `ERR unknown command` as "engine has no MSETEX" covers both quote styles and the optional args clause.
