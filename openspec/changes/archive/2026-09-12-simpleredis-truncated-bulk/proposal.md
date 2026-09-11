## Why

`readBulk` short-read and malformed-header branches have no tests. `clean == false` is what stops a misaligned socket from returning to the idle pool; without that assertion, a truncated Redis/Dragonfly reply can leak one request’s bulk payload into the next Get/MGet.

## What Changes

- Add same-package unit tests for a truncated bulk (`$100` then 40 bytes then close): command error is `redis:unreachable`, idle pool is empty, and a later Get returns that key’s own bytes. Keep `startStaticRedis`; add a helper that writes raw bytes and may close mid-stream.
- Add unit tests for `$abc\r\n` and an array element head that is neither `$`, `:`, nor `+`: error `redis:issue?`, nothing pooled. Cover `readBulk`’s non-`$` head from a same-package call; do not change `readReply`.
- Prove live Get/MGet on existing `/redis` and `/dragonfly` return the caller’s own unique value (not a shared `"ok"`). Two overlapping requests per route, distinct tokens. Keep dest Eval script (Lua 5.1-safe, `KEYS` declared). No new compose services, routes, or image pins.
- Do not edit `simpleredis.go` unless a committed test fails on dest. Out of scope: exec retry (test-05), production rewrite when dest already returns `clean == false` on short read.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: a short-read or protocol-garbage reply MUST NOT return the socket to idle; callers match `redis:unreachable` vs `redis:issue?` as today.
- `std_go_simpleredis_resp-commands`: live Get/MGet on Redis and Dragonfly MUST return the caller’s unique own-value (Pester on `/redis` and `/dragonfly`); Eval stays Lua 5.1-safe with declared `KEYS`.

## Impact

- `simpleredis/simpleredis_test.go` (raw-reply fake; idle and second-command assertions). `simpleredis.go` only if those tests fail.
- `e2e/simpleredisprobe/plugin.go` (Set payload = per-request token; Get/MGet headers). Dest Kong Eval snippet unchanged.
- `scripts/integration-tests.Tests.ps1` (own-value and overlapping requests on `/redis` and `/dragonfly`). Compose services and image pins stay.
- Main specs `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands` after archive.
