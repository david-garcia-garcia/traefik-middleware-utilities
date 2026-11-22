# Design

## Context

`tokenbucket/lua.go` `allowScript` is the Traefik `AllowTokenBucketRaw` copy at @903e8a9, with `table.maxn` already replaced by `#rl_source == 4` and an empty-hash seed of `tokens = burst`, `last = t`. See proposal.md for why that positional read fails. Explore decisions: `devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/explore.md`. Spec: `std_go_tokenbucket_lua-eval`.

## Goals / Non-Goals

**Goals:**

- Read `last` and `tokens` by name so hash field order cannot swap them.
- Keep the empty-bucket seed and every other line of the Traefik script.
- Prove a full bucket admits `burst` and denies the rest on live Redis and Dragonfly, including a tokens-first layout.

**Non-Goals:**

- Changing `tokenbucket/memory.go` or writing `allowScriptDigest` by hand.
- Rewriting refill, maxDelay, HSET, or EXPIRE.
- Updating the separate `tokenBucketScript` copy in `simpleredis/bench_test.go`.
- Forcing hashtable encoding with `CONFIG SET`.

## Decisions

1. **`HMGET` by name.** Replace `redis.call('hgetall', key)` and `if #rl_source == 4` with `redis.call('hmget', key, 'last', 'tokens')`. The bucket exists only when both values are truthy; otherwise seed `tokens = burst`, `last = t`. Same approach as traefik/traefik#14300. Alternative: sort `HGETALL` pairs in Lua — rejected; field order is not a contract. Alternative: `CONFIG SET hash-max-listpack-*` — rejected; forbidden inside a script and would change the shared server.

2. **Truthy check covers Redis and Dragonfly.** A missing field is Lua `false` on the live Redis (`127.0.0.1:16379`) and the live Dragonfly (`127.0.0.1:16380`). `nil` would also fail the truthy test. Alternative: compare to `false` only — rejected; a truthy test already matches both.

3. **Header comment records the deviation.** State the script is copied from Traefik @903e8a9 and that this read now differs, citing traefik/traefik#14297 and PR #14300. Alternative: leave the header claiming a positional `#rl_source == 4` port — rejected; that comment would be false.

4. **One self-contained live test file.** `tokenbucket/traefik_issue14297_e2e_test.go` owns `TestTraefik14297_HGetAllFieldOrder` and helpers suffixed `14297`. Cases: `control_last_first`, `tokens_first_listpack`, `tokens_first_hashtable`. Hashtable may `Skip` when the server hash seed keeps `last` first. Gate with the existing `runForEachLiveEngine` / `waitLiveClient`. Alternative: a shared helper file — rejected; parallel issues would collide on the same identifiers.

## Risks / Trade-offs

- [Script is no longer a positional port of Traefik] → Mitigation: header cites the upstream issue and PR; the rest of the script stays.
- [Hashtable subtest skips on some seeds] → Mitigation: listpack tokens-first still fails closed on both engines; the skip is logged in the test.
- [Yaegi runs the same script] → Mitigation: existing Yaegi live tests must keep passing; no new Go API.

## Migration Plan

Library bugfix. Rollback is revert. Stored hashes stay `{last, tokens}`; no key rewrite.

## Open Questions

None. Proceed policies live on `devstate/2026/11/2026-11-22-tokenbucket-hgetall-field-order/explore.md`.
