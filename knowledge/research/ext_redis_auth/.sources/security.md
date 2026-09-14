---
url: https://redis.io/docs/latest/operate/oss_and_stack/management/security/
title: Redis security
fetched: 2026-09-11
authority: official
---

Legacy authentication: requirepass in redis.conf. Unauthenticated clients are refused until AUTH with that password.

ACL (Redis 6+) is the recommended method; requirepass sets the default user password.
