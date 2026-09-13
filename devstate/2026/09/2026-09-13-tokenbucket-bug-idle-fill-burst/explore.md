# Explore
IssueKey: 2026-09-13-tokenbucket-bug-idle-fill-burst

## Concepts

- **Idle fill vs epoch refill** — Spec `std_go_tokenbucket_allow` “Idle fills to burst”: a new key is a full bucket, then consume 1. Dest implements Traefik Redis idle: `tokens=0` `last=0`, refill `elapsed = nowMicro - 0`. That is full only when `elapsed * rate >= burst`. It is empty at `Unix(0,0)` and short when burst is huge (e.g. `1e12`).
- **Two stores, one seed** — Memory `memEntry` and Lua empty `HGETALL` (`#rl_source ~= 4`) must seed the same way. Fake Redis (`tokenbucket/fake_redis_test.go`) is `consumeOne`, not a Lua interpreter; missing-hash seed must match Lua or `TestMemoryAndRedis_Agree` breaks.
- **`consumeOne` stays consume math** — Ticket: do not special-case `last=0` inside `consumeOne`. Seed full, then the existing refill/consume. Identity is not reconstructed: Allow key stays caller-owned (`std_go_tokenbucket_allow` Caller owns the key).
- **Copied Lua changes on purpose** — Traefik pin (`knowledge/research/ext_traefik_ratelimiter_token-bucket/notes.md`) uses empty hash → `tokens=0` `last=0`. This product’s Allow spec already wants a true full start. Keep the Traefik Labs MIT notice. Do not import `golang.org/x/time/rate`.
- **Usage packet** — `knowledge/devdocs/std_go_tokenbucket.md` does not say a new or TTL-expired key starts at burst. Propose/devdocsimpact owns that usage line.

```
  missing state (Memory new memEntry / Lua #rl_source ~= 4)
       │
       ├─ dest: tokens=0 last=0 → elapsed = now − epoch
       └─ agreed: tokens=burst last=now → elapsed 0, then consume 1
```

## Decisions

- Missing state is a full bucket, then consume 1. Both stores. Do not fill Memory only.
- Lua: empty `HGETALL` (`#rl_source ~= 4`) sets `tokens = burst` and `last = t`, then existing refill/consume. Keep MIT notice.
- Memory: `entry = &memEntry{}` (miss and TTL delete) starts `tokens = burst`, `last = nowMicro`. Same formulas after that.
- Fake missing hash seeds like Lua (`tokens=burst`, `last=now`) so `consumeOne` stays consume math and agreement still holds.
- Tests first: dest-adapted copy of caller `tokenbucket/repro_epoch_idle_burst_test.go` (`Allow(ctx, key)`). No `//go:build bugrepro`. `go test -short ./tokenbucket`.
- Out of scope stays out: wait mapping, NaN rate, ttl truncation, last rewind, x/time/rate, Allow signature.

Reproduced (worktree dest, throwaway then deleted):

| Case | Observed |
| --- | --- |
| Memory `epoch_clock` burst 5, `now=Unix(0,0)` | `allowed=true wait=1s tokens=-1 last=0` (want tokens>=4) |
| Memory `huge_burst_elapsed_below_burst` burst 1e12, `Unix(1_700_000_000)` | tokens `1.699999999e9`, want `burst-1=9.99999999999e11` |
| Fake Redis missing hash, same epoch clock | `allowed=true wait=1s hash last=0 tokens=-1` |

Existing `TestMemory_BurstAfterIdle` / `TestMemory_IdlePastTTLStartsFull` freeze at `Unix(1_700_000_000)` with small burst; they pass without a true full start.

## Open questions

- Q: Who already owns the client address / source identity used as the Allow key?
  Rank: additive asked — requirement Desired: opaque key; library never reads HTTP
  Decision: resolved — none in this library. Caller builds the key. This bug does not reconstruct identity.
  By: explore

- Q: Does the adapted test live as `repro_epoch_idle_burst_test.go` or fold into `limiter_test.go`?
  Rank: additive asked — Desired: copy/adapt caller `tokenbucket/repro_epoch_idle_burst_test.go`
  Decision: resolved — keep `tokenbucket/repro_epoch_idle_burst_test.go` and `TestRepro_NewKeyFillsToBurstAtEpoch` (`epoch_clock`, `huge_burst_elapsed_below_burst`). Adapt dest `Allow(ctx, key)` and dest wait return. No `//go:build bugrepro`.
  By: explore

- Q: How is Redis empty-hash fill proved under `-short` without live engines?
  Rank: additive asked — Desired: prove Redis/fake or document that Lua empty hash matches; `-short` must suffice
  Decision: assumed — three proofs, no live engines: (1) fake missing-hash seeds `tokens=burst` `last=now` like Lua, then `consumeOne`; (2) same epoch and huge-burst cases on Redis/fake as Memory; (3) `allowScript` contains the empty-hash seed (`tokens = burst` and `last = t` when `#rl_source ~= 4`). `TestMemoryAndRedis_Agree` must still pass. Document in the fake that it is consumeOne, not a Lua interpreter.
  By: explore

- Q: Does TTL-delete need its own epoch subtest?
  Rank: additive asked — Desired: new `memEntry` including after TTL delete starts full
  Decision: assumed — no extra epoch TTL subtest. One constructor: `entry = &memEntry{}` then seed burst/`nowMicro`. Epoch/huge cases prove the seed. Existing `TestMemory_IdlePastTTLStartsFull` stays as delete-then-Allow regression.
  By: explore
