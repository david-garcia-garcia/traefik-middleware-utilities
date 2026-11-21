## Context

Dest `takeExact` is `Incr` then `Expire` only when the increment is 1 (`windowcounter/limiter.go`). SimpleRedis already has `Eval` (EVALSHA then EVAL on NOSCRIPT). Buffered `flushScript` is a separate EVAL (`EXISTS` then `INCRBY`, `EXPIREAT` if new) and stays. See proposal.md for why. Spec: `std_go_windowcounter_sync-flush`. Explore: `devstate/explore.md`. `INCR` does not refresh TTL (`knowledge/research/ext_redis_incr`). After INCR the key exists, so `PTTL < 0` is `-1` (no expire).

## Goals / Non-Goals

**Goals:**
- One EVAL for exact Take increment + expire-if-no-TTL.
- Failing repro first, then EVAL, then PASS.
- Fake records Lua `EXPIRE` so `TestTake_ExpireOnFirstHit` still sees `EXPIRE … 20`.

**Non-Goals:**
- Buffered `flushScript` rewrite.
- `DEL` on expire failure.
- Refresh TTL on every hit.
- Public SimpleRedis `PTTL` verb.
- Other windowcounter bugs.

## Decisions

1. **Exact Take EVAL:** `INCR` `KEYS[1]`, then `EXPIRE` `ARGV[1]` seconds if `PTTL < 0`, return the increment. Digest at package init like `flushScript`. Alternative: client `PTTL` then `EXPIRE` — rejected; two commands still split. Alternative: expire only when increment is 1 — rejected; leftover no-TTL keys never expire.

2. **Implement order:** create `windowcounter/repro_expire_not_retried_test.go` first (port `failNextExpireCommands` / `expireCommandCount` from parent `windowcounter/fake_redis_test.go`), confirm FAIL on dest `takeExact`. Then the EVAL. Fake then implements PTTL and the exact script so a leftover no-TTL key plus a later Take records `EXPIRE`. Atomic fake EVAL does not persist INCR without the expire-if-no-ttl step.

3. **Fake TTL:** per-key expire flag. `PTTL` returns `-2` missing, `-1` exists with no TTL, remaining ms after `EXPIRE`. Exact EVAL records `lastExpire` as `EXPIRE`, key, ttl. `failNextExpireCommands` still fails standalone `EXPIRE` for the dest-failing repro. Alternative: only count client EXPIRE — rejected; EVAL would leave `TestTake_ExpireOnFirstHit` failing.

4. **Previous window stays GET** after the EVAL. Exact Take still returns Redis errors from that EVAL or GET.

## Risks / Trade-offs

- [Lost EVAL reply can double INCR] → Mitigation: SimpleRedis already accepts INCR/EVAL double-apply on lost reply; same as `flushScript`.
- [Lua `EXPIRE` is not a client EXPIRE] → Mitigation: fake records `redis.call("EXPIRE")` as `EXPIRE` argv.
- [Fail-first-EXPIRE repro cannot split EVAL] → Mitigation: ticket allows leftover-key recovery or atomic EVAL never leaving a no-TTL key; fake implements both.

## Migration Plan

No signature change. Rollback is revert. Usage packet `std_go_windowcounter.md` exact path becomes EVAL expire-if-no-TTL.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
