---
url: https://github.com/Kong/kong/blob/master/kong/plugins/rate-limiting/policies/init.lua
title: kong/plugins/rate-limiting/policies/init.lua (keys and usage)
fetched: 2026-09-11
authority: source
ref: Kong/kong@master:kong/plugins/rate-limiting/policies/init.lua
---

get_local_key = fmt("ratelimit:%s:%s:%s:%s:%s", route_id, service_id, identifier, period_date, period).
OSS redis usage realtime: EVAL INCR + EXPIRE when result_incr == 1; returns result_incr - 1.
OSS redis increment realtime (sync_rate == -1): no-op (already incremented in usage).
OSS redis increment buffered: local cur_delta += value, then rate_limited_sync at conf.sync_rate.
