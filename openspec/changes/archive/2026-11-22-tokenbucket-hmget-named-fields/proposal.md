# Proposal

## Why

`allowScript` reads the Redis hash with `HGETALL` and assigns `last` and `tokens` by position. Redis and Valkey do not guarantee hash field order, so when `tokens` comes first the bucket refills on every call and the limiter never limits. Existence of the defect was taken from upstream issue https://github.com/traefik/traefik/issues/14297 and upstream fix PR https://github.com/traefik/traefik/pull/14300. Reproduced on live Redis and Dragonfly: 10 of 10 admitted with burst 5.

## What Changes

- Replace `HGETALL` plus `#rl_source == 4` with `HMGET` of `last` and `tokens`. The bucket exists only when both values are truthy; otherwise keep the seed `tokens = burst`, `last = t`.
- Same read as upstream PR #14300. Update the `lua.go` header: copied from Traefik @903e8a9 and now different in this read, citing traefik/traefik#14297 and PR #14300. Leave the rest of the script unchanged.
- `allowScriptDigest` stays derived from `allowScript`. Do not edit `tokenbucket/memory.go`.
- Add a self-contained live test `tokenbucket/traefik_issue14297_e2e_test.go` (`TestTraefik14297_HGetAllFieldOrder`) with helpers suffixed `14297`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_tokenbucket_lua-eval`: a stored bucket is present when `HMGET` of `last` and `tokens` are both truthy, not when `HGETALL` has length 4. Empty or partial hashes still seed `tokens = burst` and `last = now`.

## Impact

- `tokenbucket/lua.go` (named read and header comment)
- `tokenbucket/traefik_issue14297_e2e_test.go` (new)
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md`
- Live Redis (`TOKENBUCKET_LIVE_REDIS`) and Dragonfly (`TOKENBUCKET_LIVE_DRAGONFLY`)
- No Memory store change, no SimpleRedis API change, no manual digest
