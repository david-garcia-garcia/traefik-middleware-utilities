# Dragonfly HMGET missing field in Lua

On the live Dragonfly used by this repo (`127.0.0.1:16380`, Lua 5.4), `HMGET` of `last` and `tokens` on a key that does not exist returns two Lua booleans `false`, the same as Redis.

Probe (2026-11-22), same EVAL as `ext_redis_hmget_missing-field`, key `tb14297:missing`:

Reply: `boolean`, `false`, `boolean`, `false`. ([.sources/live-probe.md](.sources/live-probe.md))

`rl_source[1] and rl_source[2]` is false. The empty-bucket seed runs. No Dragonfly document in this catalog states the Lua type; this probe is the owner for this server.
