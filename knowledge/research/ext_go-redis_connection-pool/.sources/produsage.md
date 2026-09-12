---
url: https://redis.io/docs/latest/develop/clients/go/produsage/
title: go-redis production usage timeouts
fetched: 2026-09-11
authority: official
---

Timeouts section documents DialTimeout / ReadTimeout / WriteTimeout only. It does not name PoolSize or PoolTimeout.

Claims default timeout is five seconds for connections and **three seconds** for reading and writing. On pin 7f3b3df, Options.init maps ReadTimeout 0 to **5s**. Follow source for this version. That also changes default PoolTimeout (ReadTimeout+1s) from the older 4s to 6s.
