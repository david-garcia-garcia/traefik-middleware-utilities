---
url: https://github.com/Kong/kong/blob/master/kong/plugins/rate-limiting/policies/init.lua
title: kong/plugins/rate-limiting/policies/init.lua
fetched: 2026-09-11
authority: source
ref: Kong/kong@master:kong/plugins/rate-limiting/policies/init.lua
---

sync_to_redis pipelines EVAL: exists check, incrby, expireat if new key.
rate_limited_sync schedules kong.timer:at(conf.sync_rate, ...).
Redis usage with sync_rate realtime uses EVAL incr + expire on first hit.
