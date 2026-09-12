# Standards

1. [hard] Leave a trail — `e2e/simpleredisprobe/plugin.go:27` — `timeWaitHoldScript` has no job comment; sibling `kongIncrbyExpireatScript` documents purpose and KEYS/Lua constraints the hold path must satisfy on Dragonfly
   → Add a one-line comment: TIME busy-wait for `?hold=` pool contention, numkeys 0, Lua 5.1-safe
   Status: done
   Argument: one-line job comment on timeWaitHoldScript (KEYS 0, Lua 5.1-safe).

```
const timeWaitHoldScript = `local start = redis.call("TIME")
local startSec = tonumber(start[1])
local startUsec = tonumber(start[2])
local need = tonumber(ARGV[1])
```
