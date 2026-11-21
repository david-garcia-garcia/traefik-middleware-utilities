# Explore
## Concepts

Exact `Take` (`sync_rate == 0`) is `takeExact`: dest `INCR` then `EXPIRE` only when the increment is `1` (`windowcounter/limiter.go`). Buffered mode is a different owner (`flushScript`: `EXISTS` then `INCRBY`, `EXPIREAT` only when the key did not exist). This ticket is only the exact path.

Redis `INCR` leaves any existing expire untouched (`knowledge/research/ext_redis_incr/notes.md`). After a failed client `EXPIRE`, the key exists with count ≥ 1 and **no TTL**. A later `INCR` returns 2+ and dest never expires. The window cannot empty.

```
Take 1: INCR → 1, EXPIRE fails → key=1, TTL none
Take 2: INCR → 2, skip EXPIRE (current != 1) → key lives forever
```

Agreed cause-fix: one EVAL on `KEYS[1]`: `INCR`, then `EXPIRE` if `PTTL < 0` (no TTL), not only when increment is 1. Do not refresh TTL when PTTL is already set. Do not `DEL` on expire failure. `flushScript` unchanged.

`PTTL` after `INCR`: the key exists, so `-2` (missing) cannot happen. `-1` is no expire; a non-negative value is remaining milliseconds. `PTTL < 0` is “no TTL”. Lua `EXPIRE` is not a client `EXPIRE`; the unit fake must record it for `TestTake_ExpireOnFirstHit`.

SimpleRedis already has `Eval` (EVALSHA then EVAL on NOSCRIPT). No public `PTTL` verb. Identity is the caller’s opaque key (`std_go_windowcounter_sliding-take`); this change does not reconstruct client address.

Measured dest skip: throwaway `TestExplore_SeededKeySkipsExpire` — `Incr` the current-window key to 1 with no expire, then `Take`. `lastExpireCommand()` was empty. File deleted. Parent checkout already has `failNextExpireCommands` / `expireCommandCount` and `repro_expire_not_retried_test.go`; dest fake does not.

## Decisions

- Exact Take becomes one EVAL (INCR + EXPIRE-if-no-TTL). Buffered `flushScript` is not rewritten.
- Implement order is required: failing repro first, then EVAL, then PASS.
- Spec `std_go_windowcounter_sync-flush` “TTL when increment returns 1” and usage “INCR + EXPIRE on first hit” move with Take to expire-if-no-TTL.
- Keep `TestTake_ExpireOnFirstHit`. Fake EVAL of the exact script records `EXPIRE` argv (TTL 20) the same way dest records a client EXPIRE.

## Open questions

- Q: After EVAL, how does `TestRepro_ExactExpireNotRetriedAfterFailure` still prove a later Take sets TTL?
  Rank: additive asked — requirement Desired names both proofs (later Take still sets TTL, or atomic EVAL never leaves a no-TTL key)
  Decision: assumed — create the dest-failing repro first: port `failNextExpireCommands` / `expireCommandCount`, fail the first EXPIRE, second Take must send EXPIRE (FAIL on unfixed `takeExact`). After EVAL, the fake implements PTTL plus the exact script: INCR then EXPIRE if no TTL, recording that EXPIRE. A leftover no-TTL key (seed count ≥ 1, or dest-style failed first expire) plus a later Take must record EXPIRE. Atomic fake EVAL does not persist INCR without the expire-if-no-ttl step. Do not keep a split-command-only assertion that EVAL can never satisfy.
  By: explore

- Q: Does the unit fake store TTLs and serve PTTL, or only record EXPIRE inside EVAL?
  Rank: additive asked — requirement Desired says the fake may need PTTL / EVAL expire-if-no-ttl
  Decision: assumed — store a per-key expire flag (or expire-at). `PTTL` returns `-2` missing, `-1` exists with no TTL, remaining ms when EXPIRE ran. Exact EVAL matches that. Wall-clock TTL decay is not required for the unit repro. `TestTake_ExpireOnFirstHit` keeps asserting `EXPIRE … 20` from that recording.
  By: explore

- Q: Does `TestTake_ExpireOnFirstHit` still see a standalone client `EXPIRE` after Take moves to EVAL?
  Rank: additive asked — requirement Tensions: keep the test; do not drop it
  Decision: assumed — no client EXPIRE. Fake EVAL of the exact script sets `lastExpire` to `EXPIRE`, key, ttl seconds so the existing test still passes. Buffered `flushScript` EVAL still records `EXPIREAT` only.
  By: explore

- Q: Who owns identity for the window key?
  Rank: additive asked — sliding-take: callers own the opaque key; library MUST NOT read HTTP
  Decision: resolved — reuse the caller’s opaque key; this change does not set or reconstruct client address, user, tenant, or Host.
  By: explore

- Q: Is a new `ext_redis_pttl` research folder required before implement?
  Rank: additive incidental — PTTL is the agreed detector; existing `ext_redis_incr` already states INCR does not refresh TTL
  Decision: assumed — do not write a research folder. Cite `ext_redis_incr` (TTL untouched) and official PTTL (`-1` no expire, `-2` missing) in the change design. After INCR the key exists, so `PTTL < 0` is `-1`.
  By: explore
