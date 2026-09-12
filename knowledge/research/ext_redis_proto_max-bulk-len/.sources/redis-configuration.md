---
url: https://redis.io/docs/latest/operate/oss_and_stack/management/config/
title: Redis configuration
fetched: 2026-09-12
authority: official
---

The list of configuration directives, with comments describing meaning and usage, is the self-documented sample `redis.conf` shipped with Redis.

`CONFIG SET` / `CONFIG GET` reconfigure most directives at runtime. Changes do not rewrite `redis.conf` unless `CONFIG REWRITE` is used. Fields left at the default are not added to the file.
