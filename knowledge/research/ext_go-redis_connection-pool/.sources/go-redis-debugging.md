---
url: https://redis.uptrace.dev/guide/go-redis-debugging.html
title: Debugging Go Redis: pool size, timeouts
fetched: 2026-09-11
authority: official
---

README on this pin labels redis.uptrace.dev "old documentation"; it is still the page that names PoolSize and the pool-timeout error.

Default pool size: 10 connections per every available CPU (`runtime.GOMAXPROCS`).

Error `redis: connection pool timeout` when there are no free connections in the pool for `Options.PoolTimeout`. Also when Redis is slow and every pool connection is blocked longer than PoolTimeout. Mentions unreleased PubSub/Conn as a cause.

Does not state the numeric PoolTimeout default.
