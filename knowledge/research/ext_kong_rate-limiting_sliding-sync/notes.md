# Kong Rate Limiting Advanced — sliding window and sync_rate

Facts for implementing a Kong-aligned distributed window counter (not the OSS fixed-window plugin).

## Sliding window (Advanced default)

- `window_type` default is `sliding`. Sliding windows weight the immediately preceding window using elapsed time within the current window ([Kong window types](https://developer.konghq.com/gateway/rate-limiting/window-types)).
- Fixed windows map each request to one static bucket; at a boundary burst can admit 2× the configured rate in a short span. Sliding windows move the effective limit continuously to match configured rate ([Kong window types](https://developer.konghq.com/gateway/rate-limiting/window-types)).
- Ticket locks v1 formula: `estimated = current + previous × (1 − elapsed/window)`.

## sync_rate (Advanced redis strategy)

- `sync_rate = 0`: synchronous behavior — counters hit Redis on each increment; most accurate ([Kong Rate Limiting Advanced](https://developer.konghq.com/plugins/rate-limiting-advanced)).
- `sync_rate > 0`: sync interval in seconds; lower values sync more often. Minimum allowed interval is **0.02 s (20 ms)** per plugin schema ([Kong Advanced schema](https://developer.konghq.com/plugins/rate-limiting-advanced)).
- `sync_rate = -1`: counters stay in node memory only (not applicable to this library's Redis-backed design).

## OSS redis flush script (reference for buffered mode)

Kong OSS `rate-limiting` redis policy batches local deltas and flushes with EVAL INCRBY + conditional EXPIREAT when the key did not exist ([Kong/kong policies/init.lua@master](https://github.com/Kong/kong/blob/master/kong/plugins/rate-limiting/policies/init.lua)):

```lua
local key, value, expiration = KEYS[1], tonumber(ARGV[1]), ARGV[2]
local exists = redis.call("exists", key)
redis.call("incrby", key, value)
if not exists or exists == 0 then
  redis.call("expireat", key, expiration)
end
```

OSS `sync_rate != -1` path uses `rate_limited_sync` timer at `conf.sync_rate` seconds; local admit uses `cur_usage + cur_delta` until flush ([same file](https://github.com/Kong/kong/blob/master/kong/plugins/rate-limiting/policies/init.lua)).

OSS exact path (`sync_rate == -1` / realtime) uses EVAL INCR + EXPIRE on first hit per request ([same file](https://github.com/Kong/kong/blob/master/kong/plugins/rate-limiting/policies/init.lua)). Ticket maps exact mode to `sync_rate=0` with `Incr` every `Take`.

## Dragonfly constraints (shared scripts)

- EVAL scripts must declare touched keys in `KEYS`; avoid `table.maxn` (Lua 5.4). See `knowledge/research/ext_dragonfly_eval/notes.md`.
