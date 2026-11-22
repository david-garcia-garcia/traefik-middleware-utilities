# Redis HMGET missing field in Lua

On the live Redis used by this repo (`127.0.0.1:16379`), `HMGET` of `last` and `tokens` on a key that does not exist returns two Lua booleans `false`.

Probe (2026-11-22), EVAL with one key `tb14297:missing`:

```lua
local r = redis.call('hmget', KEYS[1], 'last', 'tokens')
return {type(r[1]), tostring(r[1]), type(r[2]), tostring(r[2])}
```

Reply: `boolean`, `false`, `boolean`, `false`. ([.sources/live-probe.md](.sources/live-probe.md))

A truthy test of both elements (`rl_source[1] and rl_source[2]`) is false, so the token-bucket script takes the empty-bucket seed.
