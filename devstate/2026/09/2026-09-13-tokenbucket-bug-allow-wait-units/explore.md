# Explore

## Concepts

- **Allow** (`tokenbucket.Memory.Allow`, `tokenbucket.Redis.Allow`): one consume of an opaque key. Dest returns `(bool, time.Duration, error)`. The library does not sleep. Callers own the key (`openspec/specs/std_go_tokenbucket_allow/spec.md`).
- **Refund**: `consumeOne` and Traefik Lua restore the token when `waitMicro > float64(maxDelay.Microseconds())` / `wait_duration > max_delay`. ARGV is already whole microseconds (`tokenbucket/clock.go`, `tokenbucket/lua.go`).
- **Admit (dest)**: `waitDuration(waitMicro) <= maxDelay` on a full `time.Duration`. `waitDuration` truncates sub-µs waits to 0 and overflow-casts large waits to `MinInt64`.
- **Fail-open**: refund true and admit true at once. Tokens come back; the next `Allow` repeats forever.

```
burst spent
    │
    ▼
consumeOne / Lua          Go Allow
waitMicro > maxDelayµs   waitDuration(waitMicro) <= maxDelay
    │                              │
    ▼                              ▼
 refund token                   allowed=true
    └──────── both true ─────────┘
              fail-open loop
```

## Decisions

- One comparison for admit and refund, in microseconds: `allowed` iff `waitMicro <= float64(maxDelay.Microseconds())`. Lua already refunds on `>`; equal wait still admits.
- `Memory.Allow` and `Redis.Allow` both use that. Redis already parsed `waitMicro`; do not convert to `time.Duration` to decide the bool.
- Leave Lua as Traefik’s script. Leave ARGV as `maxDelay.Microseconds()`.
- Delete `allowedFromWait`. Delete named `waitDuration`. Produce the Duration return by inlining the conversion at the two `Allow` sites so dest’s `(bool, time.Duration, error)` stays (`std_go_tokenbucket_allow`).
- Do not round maxDelay up, skip refund, or special-case 1500ns.
- Tests first: the four dest-failing cases (1500ns + rate `1e6/1.2`; 1µs + rate 999999; maxDelay=0 + rate `1e12`; rate `1e-12` overflow), then the how, then pass. Adapt caller repros to dest’s `Allow(ctx, key)` arity without weakening.
- Bound: this fail-open only. Not Eval nan/Inf wait or other tokenbucket bugs.

Measured on dest `9c2a11a` (throwaway tests, then deleted): all four cases failed.

| Case | Observed |
| --- | --- |
| rate `1e6/1.2`, maxDelay 1500ns | `waitMicro=1.2` > `maxDelayMicro=1` (refund) and `wait=1.2µs <= 1.5µs` (admit). `second=true`, `tokensAfterSecond=0`, `third=true` |
| rate 999999, maxDelay 1µs | `waitMicro=1.000001000001` > 1; `waitDuration` truncates to 1µs; same fail-open |
| maxDelay=0, rate `1e12` | second Allow admitted (`waitDuration` truncates `1e-6` µs to 0) |
| rate `1e-12`, empty bucket | `waitDuration` overflow-casts to negative; Allow admitted vs hour maxDelay |

## Open questions

- Q: After deleting `waitDuration`, how is Allow's Duration return produced?
  Rank: additive asked — Desired names delete `waitDuration`; dest `std_go_tokenbucket_allow` already returns wait duration (existing callers keep working)
  Decision: resolved — keep `(bool, time.Duration, error)`; inline `waitMicro * time.Microsecond` at `Memory.Allow` and `Redis.Allow` returns only; admit bool is microseconds, not that Duration.
  By: implement

- Q: Who already owns the request identity used as the Allow key?
  Rank: additive asked — `std_go_tokenbucket_allow` names the caller-owned opaque key
  Decision: resolved — the caller already owns the key; `Allow` does not reconstruct client address, user, tenant, or Host.
  By: explore
