---
url: https://redis.io/docs/latest/commands/select/
title: SELECT
fetched: 2026-09-11
authority: official
---

SELECT index — zero-based logical database. New connections use database 0.

Success return: simple string OK.

Cluster: SELECT cannot be used (database zero only). Dest e2e is standalone redis:7-alpine, not cluster.
